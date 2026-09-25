# Ruling — sign-in through a Keycloak broker, zero clicks (OD-30)

Date 2026-09-25. Status: **RULED**. Every later agent applies this without re-arguing it.
Source decision: `docs/superpowers/planning/2026-09-23-owner-decisions.md:166` (OD-30).
Evidence: `.superpowers/sdd/identity/facts-panacea-keycloak.md` (Panacea's realm, read-only survey)
and `.superpowers/sdd/identity/facts-app-identity.md` (every PanaceaApp place that assumed direct Entra).
Worktree citations are relative to `.claude/worktrees/<tree>/`. Panacea paths are relative to
`Projects/Panacea` (READ-ONLY, OD-13; nothing copied).

## 1. The decision and what it supersedes

**Users sign in to PanaceaApp through a Keycloak realm that brokers to Entra, with no clicks.**
No new Entra app registration is made for sign-in, and **no IT ticket is sent for sign-in**.

- **The Entra registration is the one Panacea's Keycloak already brokers through**: client id
  beginning `37f8bd70-`, set identically in every Panacea estate file (`deploy/environments/panacea-dev.env:151`,
  `panacea-sandbox.env:84`, `panacea-aue.env:148`). Panacea's env files name it
  `AU-PROD-Panacea-RITM6316333` (`panacea-dev.env:140-142`); `deploy/f3-entra-cutover.sh:100` labels the
  same variable "the 'Panacea Platform API' app registration". OD-30's "Panacea Platform API" means
  **this client id**, whatever its display name (see section 12, U-1).
- **Why no IT ticket:** a new Keycloak client adds no Entra redirect URI. Entra only ever sees
  Keycloak's broker callback `https://auth.<domain>/realms/panacea/broker/entra/endpoint`, which is
  already registered (`panacea-dev.env:141-146`, `:144`). PanaceaApp's loopback redirect is registered
  on the Keycloak client, never on Entra.
- **What is left of the IT request:** Part 2 only (phase 8's Azure plane: state storage, AKS Entra
  RBAC groups, Postgres admins; `docs/superpowers/planning/2026-09-24-it-request-entra.md:56-93`). That is
  workload and admin identity, not user sign-in, and OD-30 leaves it unchanged. It is sent when phase 8
  provisions, not now.

**Superseded, each by this ruling:**

| Superseded text | Where | What replaces it |
|---|---|---|
| OD-6 "directly against Entra" (the half that names the issuer) | `owner-decisions.md:17` | Auth-code + PKCE against a **Keycloak realm** that brokers Entra (section 2). OD-6's other half stands: system browser, tokens in Windows Credential Manager, users always themselves, nothing pooled. |
| ADR-H1 §5.2 "runs PKCE against Entra directly ... No broker" and §6.1 "A broker (Keycloak) in front of Entra — Rejected" | hub `docs/superpowers/adr/ADR-H1-hub-architecture.md:144-146`, `:310-396`, `:598-607` | §5.2's mechanics stand (one-shot `127.0.0.1:0` listener, system browser, refresh token in Credential Manager, renderer never holds a token). The issuer, token profile and validation table change (sections 2-5). §6.1 is reversed. |
| Phase 8 OQ-4 "No server-side identity broker (Keycloak) in v1" | env `docs/superpowers/plans/2026-09-23-phase-8-own-environment.md:123` | A broker in v1: Panacea's realm now, phase 8's own Keycloak later (section 9). |
| Phase 8 P8-D4 "validates an Entra access token (issuer = our tenant, audience = the broker API, scope `Sandboxes.Access`), owner = `tid` + `oid`" | same file `:134` | The broker validates a **realm** access token under the `keycloak` claims profile: audience `panacea-app-broker`, owner = `oid` (section 4). The owner-label rule stands. |
| IT request Part 1 (items 1-7: tenant id, *PanaceaApp Desktop*, *Hub API*, *Sandbox Broker* registrations, `Hub.OrgAdmin`, CA note, ADO sign-in) | `2026-09-24-it-request-entra.md:1-54` | Nothing is asked of IT for sign-in. The Keycloak asks go to Panacea's own update (section 10). |
| Hub Q5 "Azure DevOps uses the same Entra sign-in ... no ADO PAT" | hub ADR-H1 `:743`; `2026-09-24-it-request-entra.md:104` | **Flipped:** Azure DevOps is a personal access token per user (section 7). |

## 2. The chain

```
desktop renderer --(loopback)--> control plane --(system browser)--> Keycloak realm --(broker)--> Entra
                                      ^                                   |  auto-link           (KPMG tenant,
                                      +--- 127.0.0.1:<port>/callback -----+                       existing reg)
```

1. **Trigger.** At launch the control plane first tries a silent refresh (section 3). Only if that
   fails, or the renderer calls `POST /v1/auth/sign-in`, does it run the interactive flow below.
2. **PKCE.** The control plane makes a verifier (32 random bytes, base64url), its **S256** challenge,
   a `state` and a `nonce`, and opens a **one-shot listener on `127.0.0.1:0`** (RFC 8252 §7.3). ADR-H1
   §5.2's reason for a separate listener (no side-effecting GET on the main listener) stands.
3. **Browser.** The **control plane itself opens the system browser** (Windows `ShellExecuteW` "open"
   on the URL, through `golang.org/x/sys/windows`), so a launch-time re-sign-in needs no renderer round
   trip and no click. `POST /v1/auth/sign-in` still answers `{ "authorizeUrl": ... }`; the renderer shows
   it only as a fallback link ("Open the sign-in page") if no callback arrives in 15 seconds. No embedded
   webview ever sees the Entra page (RFC 8252 §8.12; Entra conditional access expects this).
4. **Authorize request** to the realm's `authorization_endpoint` (from
   `<issuer>/.well-known/openid-configuration`, cached for the process):
   `response_type=code`, `client_id=panacea-app-desktop`,
   `redirect_uri=http://127.0.0.1:<port>/callback`,
   `scope=openid offline_access panacea-app-hub panacea-app-broker panacea-app-gateway panacea-app-fleet`,
   `state`, `nonce`, `code_challenge`, `code_challenge_method=S256`, `kc_idp_hint=entra`,
   `login_hint=<Windows UPN>`. Nothing else (section 3 says why no `prompt`, no `domain_hint`).
5. **Broker.** Keycloak skips its own page (`kc_idp_hint`), redirects to Entra's tenant-specific
   authorize endpoint through the IdP alias `entra` (Panacea `deploy/portal/setup-keycloak.sh:395`,
   `:482`, `:489`), with the IdP's own confidential credentials (`client_secret_post`, `:488`). Entra
   answers Keycloak's broker endpoint; the IdP's `tid` filter refuses any other tenant (`:509`);
   first-broker-login creates or auto-links the Keycloak user (`:434-464`).
6. **Callback.** Keycloak redirects to the loopback with `code` and `state`. The listener checks
   `state`, answers the static page (section 3), closes, and exchanges `code` + verifier at the
   `token_endpoint` as a **public client** (no secret on the laptop). The control plane checks the
   id_token's `nonce`, `iss`, `aud == panacea-app-desktop` and `azp`, reads `oid`/`tid`/`name`/`email`
   from it for `/v1/auth/status`, and never sends the id_token anywhere.
7. **Tokens.** The **offline refresh token** goes to Windows Credential Manager (OD-6). The
   code-exchange access token (which carries every audience) is **discarded, never sent**. Each call
   site asks `TokenSource.Token(ctx, "<audience scope>")`, which refreshes with
   `scope=openid <audience scope>` to get a **single-audience** access token, cached in memory until
   60 seconds before `exp`. The renderer never holds any token.

**Why `offline_access`.** Panacea's realm runs one replica with in-memory sessions (`deploy/main.tf:928`,
`:941-957`) and prod keeps the 10-hour SSO maximum (`setup-keycloak.sh:157-158`). A plain refresh token
dies with its SSO session, on a pod move or overnight. An offline token lives in Keycloak's database
(offline idle default 30 days, facts §6) and survives both, so next-morning launches stay silent.

**The issuer is configuration.** One variable names it; nothing in code names a host.

| Variable | Default | Values |
|---|---|---|
| `PANACEA_APP_OIDC_ISSUER` | `http://127.0.0.1:<devissuer port>/realms/panacea-app-dev` (local) | Panacea dev realm `https://auth.dev.panacea.kpmg/realms/panacea` until phase 8; then phase 8's own `https://auth.<our domain>/realms/panacea-app` |
| `PANACEA_APP_OIDC_PROFILE` | `keycloak` | `keycloak`, or `entra` for a later direct-Entra issuer (section 5) |
| `PANACEA_APP_OIDC_CLIENT_ID` | `panacea-app-desktop` | the desktop's public client id |
| `PANACEA_APP_OIDC_IDP_HINT` | `entra` | the realm's Entra IdP alias; empty disables the hint (facts §8) |

These **replace** the Hub contract's `PANACEA_APP_ENTRA_AUTHORITY` and `PANACEA_APP_ENTRA_CLIENT_ID`
(hub `docs/superpowers/contracts/2026-09-23-hub-contract.md:1236-1262`, `:2146-2192`) and phase 8's
`PANACEA_APP_BROKER_ENTRA_AUTHORITY` (env `.../phase-8-own-environment-parts/task-17.md:33`). Servers read
the same `PANACEA_APP_OIDC_ISSUER` and `PANACEA_APP_OIDC_PROFILE`, plus their own audience (section 4).

**Before phase 8, the realm is Panacea's dev realm**, not prod: Panacea's updates land in dev first and
its SSO lifetimes are set there (`deploy/environments/panacea-dev.env:131-133`). Prod
(`auth.panacea.kpmg`) is not used by PanaceaApp. **Locally, `devissuer` runs the identical flow**: it
serves Keycloak's path layout (`/realms/<realm>/.well-known/openid-configuration`,
`/realms/<realm>/protocol/openid-connect/auth|token|certs`), honours `kc_idp_hint` as a no-op, and
emits the `keycloak` profile (section 5), so the control plane's code path is the same byte for byte.

## 3. Zero clicks, exactly

Each screen that could appear, and what removes it:

| Screen | Removed by |
|---|---|
| Keycloak's login page (username/password + "Microsoft (KPMG)" button) | **`kc_idp_hint=entra`** on every authorize request. Needs no realm change: the hint overrides the Identity Provider Redirector and Panacea sets no redirector (facts §4, §8). |
| Entra home-realm discovery | Nothing to remove: the IdP already calls the **tenant-specific** endpoint `login.microsoftonline.com/<KPMG tenant>/...` (`setup-keycloak.sh:482`, `:489`). **`domain_hint` is not sent**: Keycloak 25.0.0 cannot forward it (forwarded query parameters exist on `main` only, facts §8) and it would add nothing. |
| Entra account picker / password | The user's **Windows sign-in session** (the device's Primary Refresh Token, used by Edge natively and by Chrome through the Windows Accounts extension) plus **`login_hint=<UPN>`**, forwarded by the IdP's "Pass login_hint" switch (config `loginHint: "true"`, a Panacea ask, section 10; today it is off, facts §3). |
| Entra consent | None expected: the registration is already consented for `openid profile email` by Panacea's use (`defaultScope`, `setup-keycloak.sh:508`). |
| Keycloak "review profile" / "link account" | Panacea's own flow `panacea first broker login` = `idp-create-user-if-unique` or `idp-auto-link` (`setup-keycloak.sh:412`, `:434-464`); it has no review-profile step and auto-links an existing user by email (`trustEmail:true`, `:485`). |
| Re-authentication at every launch | The **offline refresh token** in Credential Manager, redeemed silently at launch (below). |

**No `prompt` parameter.** The desktop never sends `prompt`, and the IdP's `prompt` setting stays unset
(facts §3), so neither Keycloak nor Entra is told to re-ask.

**Reading the Windows UPN.** The control plane calls `GetUserNameExW(NameUserPrincipal)`
(`windows.GetUserNameEx(windows.NameUserPrincipal, ...)` in `golang.org/x/sys/windows`, the same call
`whoami /upn` makes), once per process. If it fails (for example `ERROR_NONE_MAPPED` on a device where the
local account has no UPN), `login_hint` is **omitted**, never guessed; Entra then relies on the browser
session alone. `PANACEA_APP_OIDC_LOGIN_HINT` overrides it (tests, and a user whose Entra UPN differs from
the Windows one). On non-Windows builds the hint is omitted.

**Silent refresh at every launch.** On start the control plane reads the refresh token from Credential
Manager and redeems it for a `panacea-app-hub` access token before the renderer's first status poll.
Success: signed in, no browser. `invalid_grant` (revoked, expired, realm reset): delete the stored token
and **run the interactive flow at once, without a click**; with the rows above it completes on its own and
the browser tab is the only thing the user sees. Network failure: stay signed-out-offline and retry on the
heartbeat; never open a browser for a network error.

**The loopback page.** The listener answers `200`, `Content-Type: text/html; charset=utf-8`,
`Cache-Control: no-store`, a static page with no script and no external resource, whose whole visible
text is: **"Signed in. You can return to PanaceaApp."** On a `state` mismatch or an `error` parameter it
answers `400` with "Sign-in did not complete. Return to PanaceaApp to try again." and never echoes the
error text into HTML.

**Bringing the app to the front.** When `/v1/auth/status` turns `signedIn`, the renderer calls Tauri's
`getCurrentWindow().unminimize()` then `.setFocus()`. Windows may refuse a background process the
foreground (foreground lock); the renderer then calls `.requestUserAttention(UserAttentionType.Informational)`,
which flashes the taskbar button. It never steals focus a second time.

**What can still prompt, and is correct to:**
- **Conditional-access MFA**: sign-in frequency, a risky sign-in, or a new location. KPMG policy wins.
- **A non-compliant or unregistered device, or a browser without the Windows session** (a default browser
  other than Edge with no Windows Accounts extension): Entra shows its sign-in page. Not worked around.
- **A Keycloak required action** set on the user by the realm admin (terms, profile update). None is
  configured by Panacea's script; the live realm is UNVERIFIED (section 12).

**Why WAM plus token exchange is NOT used now.** Windows' Web Account Manager would get an Entra token
from the PRT with no browser tab at all, but (1) it needs a broker redirect URI
(`ms-appx-web://Microsoft.AAD.BrokerPlugin/<client id>`) on the Entra registration, which is an IT change
to an IT-owned registration, the very ticket OD-30 removes; (2) turning that Entra token into a realm token
needs Keycloak **token exchange** (external to internal), a preview feature Panacea's Keycloak does not
enable; (3) it adds a native WAM binding to the control plane. The system-browser flow already has zero
clicks. Revisit only in phase 8, on our own Keycloak, if the owner judges the brief browser tab a problem.

## 4. The token profile (`keycloak`)

**The person key is `(tid, oid)`: Entra's tenant id and object id, carried through Keycloak as claims.**
Never `sub`. Keycloak's `sub` is its own user id and changes if the user is re-created in another realm;
Entra's `oid` is fixed per person per tenant across every app. Carrying `oid` means the key survives the
move to phase 8's own Keycloak (same registration, same `oid`) and a later move to direct Entra (native
`oid`). It also means every BUILT consumer of `(tid, oid)` stands unchanged: `Claims.Key()`
(env `control-plane/internal/entra/entra.go:56-58`), `sandbox.OwnerLabelValue(tenantID, objectID)`
(env `control-plane/internal/sandbox/manager.go:138-141`), `hubapi.Org.TenantID` (hub
`control-plane/internal/hub/hubapi/types.go:104`), the Hub schema `(entra_tenant_id, entra_oid)` and
phase 6's `wire.Owner{TID, OID}`.

**Access-token claims a validator relies on:**

| Claim | Value | Rule |
|---|---|---|
| `iss` | exactly `PANACEA_APP_OIDC_ISSUER` | exact string compare |
| `typ` | `Bearer` | required; refuses id and refresh tokens |
| `aud` | string or array, **contains** the service's audience: `panacea-app-hub`, `panacea-app-broker`, `panacea-app-gateway` or `panacea-app-fleet`. May also hold Keycloak's `account`. | contains, not equals |
| `azp` | `panacea-app-desktop` for a person; a service client id for a service caller (section 6) | in the service's allowlist |
| `oid` | Entra object id, GUID | **required** for a person token; absent on a service token |
| `tid` | Entra tenant id, GUID | required for a person; must be an enrolled tenant (the Hub's org key) |
| `scope` | space-separated; contains the audience scope's name | Keycloak's `scope` replaces Entra's `scp` |
| `roles` | flat array of realm role names; the Hub reads **`hub.org_admin`** | replaces the Entra app role `Hub.OrgAdmin` (hub ADR-H1 `:430`) |
| `email`, `email_verified` | from Entra through Panacea's `email` IdP mapper, `trustEmail:true` | the Hub's invite match (contract `:318-319`, unchanged) |
| `name`, `preferred_username` | `preferred_username` = the email (Panacea's username mapper, `setup-keycloak.sh:568-579`) | display only |
| `sub` | Keycloak user id | logged, never a key |
| `groups` | **absent**: the desktop does not request Panacea's optional `groups` scope | Hub membership is the Hub's own (ADR-H1 §6.6 stands) |

`hub.org_admin` is a **realm role** assigned by the realm's admin (Panacea's team now, ours in phase 8),
not by IT. Scopes `Hub.Access` and `Sandboxes.Access` no longer exist; the audience scopes replace them.

**Every mapper, and nothing else is added:**

| # | Where | Type | Config |
|---|---|---|---|
| M1 | IdP `entra` | `oidc-user-attribute-idp-mapper` | claim `oid` -> user attribute `entra_oid`, **syncMode `FORCE`** |
| M2 | IdP `entra` | `oidc-user-attribute-idp-mapper` | claim `tid` -> user attribute `entra_tid`, **syncMode `FORCE`** |
| UP | realm user profile | attribute declarations | `entra_oid`, `entra_tid`: view `admin`, edit `admin` only |
| P1 | client `panacea-app-desktop` (dedicated scope) | `oidc-usermodel-attribute-mapper` | `entra_oid` -> claim `oid`, String; access token + id token |
| P2 | same | `oidc-usermodel-attribute-mapper` | `entra_tid` -> claim `tid`, String; access token + id token |
| P3 | same | `oidc-usermodel-realm-role-mapper` | claim `roles`, multivalued; access token only |
| A1-A4 | client scopes `panacea-app-hub`, `-broker`, `-gateway`, `-fleet`, each **optional** on the desktop client, `include.in.token.scope: true` | `oidc-audience-mapper` | `included.custom.audience` = the scope's own name; access token only |

Why each non-obvious setting:
- **`FORCE`, not the IdP's `IMPORT`** (`setup-keycloak.sh:495-507`): Panacea's users already exist in the
  realm. An `IMPORT` mapper writes only when a user is first created, so every existing user would never
  get `entra_oid`. `oid` and `tid` never change, so `FORCE` rewrites the same value at each login.
- **UP, admin-only**: Keycloak 24 and later always run the declarative user profile; an undeclared attribute
  is "unmanaged", which a realm may drop. Declaring it admin-edit-only also stops a user rewriting their own
  `entra_oid` in the account console, which would let them become someone else.
- **Audience scopes, optional**: the authorize request asks for all four; each refresh narrows to one
  (section 2, step 7), so a token given to the broker cannot be replayed at the Hub (the Hub contract's
  confused-deputy defence, `:2070`, kept).
- Keycloak's built-in scopes (`basic`, `profile`, `email`, `roles`, `web-origins`, `acr`) are left as the
  realm has them (facts §5). P3's flat `roles` duplicates `realm_access.roles` so both profiles read one
  claim name.

## 5. The validator

env task 16 BUILT `control-plane/internal/entra` (commits `669204b` .. `1772f25`); it has no importer yet
(facts-app-identity §2a), and every Entra-only rule in it (tenant first, `ver`, `iss .../<tid>/v2.0`, GUID
`aud`/`azp`, `scp`, the `/discovery/v2.0/keys` path) refuses every Keycloak token.

**Ruling: the package is renamed `control-plane/internal/oidcauth` and gains a claims profile.** One
package, two profiles (`keycloak`, `entra`), so a later direct-Entra issuer is configuration, not code.
The rename is a `git mv` (history kept); a package named `entra` validating Keycloak tokens would mislead
every reader. The IdP-neutral parts (RS256 only, alg refusal, size limits, exp/nbf skew, JWKS cache and
refresh, `Middleware`, `ClaimsFrom`) are kept as they are.

**Exported API change** (everything not listed is unchanged):

```go
package oidcauth

type Profile string
const (
    ProfileKeycloak Profile = "keycloak"
    ProfileEntra    Profile = "entra"
)

type Config struct {
    Profile        Profile  // NEW, required; New refuses "". Callers default it from PANACEA_APP_OIDC_PROFILE.
    Issuer         string   // NEW, keycloak: exact iss; JWKS at Issuer + "/protocol/openid-connect/certs".
    AuthorityHost  string   // entra only, unchanged.
    TenantIDs      []string // both: enrolled tids, GUIDs. keycloak: checked AFTER the signature.
    Audience       string   // keycloak: a client-id string, aud must CONTAIN it. entra: GUID, unchanged.
    RequiredScope  string   // keycloak: in `scope`. entra: in `scp`, unchanged.
    AllowedClients []string // person clients (azp). keycloak: strings. entra: GUIDs, unchanged.
    ServiceClients []string // NEW, keycloak only: azp values accepted with NO oid and NO tid (section 6).
    ClockSkew time.Duration; AllowInsecureAuthority bool // now also permits a loopback http Issuer
    HTTPClient *http.Client; Now func() time.Time
}

type Claims struct { // existing fields kept; NEW: Issuer, Subject string; Service bool
    ... ; Issuer, Subject string; Service bool
}
func (c Claims) Key() string // person: "tid/oid" (unchanged). service: "client/" + ClientID.

var ErrType = errors.New("oidcauth: typ is not Bearer") // NEW; keycloak profile only
```

**`keycloak` profile order:** size and alg -> signature against the issuer's JWKS -> exp/nbf -> `iss` equals
`Issuer` -> `typ == "Bearer"` -> `aud` contains `Audience` -> `azp` in `AllowedClients` or `ServiceClients`
-> `RequiredScope` in `scope` -> person: `oid` a GUID and `tid` a GUID in `TenantIDs`; service: `oid` and
`tid` both absent, `Service = true`. `ErrVersion` is never returned under `keycloak`. `New` refuses:
an empty `Profile`, a non-https `Issuer` (a loopback http one only with `AllowInsecureAuthority`),
`ServiceClients` under `entra`, and a client id in both lists.

**Tests that change:**
- `entra_test.go`, `jwks_test.go`, `testkey_test.go`, `middleware_test.go` move with the package; every
  `Config` literal in the existing suite gains `Profile: ProfileEntra`. Their assertions stay: the Entra
  profile's behaviour is pinned exactly as task 16 built it.
- `testkey_test.go` gains a Keycloak token builder (`iss`, `typ`, array `aud`, string `azp`, `scope`,
  `oid`, `tid`, `roles`).
- NEW `keycloak_test.go`, one row per rule: a good person token; wrong `iss`; `typ` `ID`; `aud` as string
  and as array; `aud` missing; `azp` not allowed; person token with no `oid`; `tid` not enrolled
  (`ErrTenant`), and a badly signed token with a foreign `tid` returns `ErrSignature` (order pinned);
  `scope` without `RequiredScope`; a service token (Fleet) accepted with `Key() == "client/panacea-app-fleet"`;
  a service token carrying `oid` refused; each `New` refusal above.
- NEW cross-profile rows: a Keycloak token under `ProfileEntra` is refused, an Entra token under
  `ProfileKeycloak` is refused (`ErrIssuer`).
- `jwks_test.go` gains the `/protocol/openid-connect/certs` path row.

**Who does it:** **env task 17** (`cmd/broker` authentication), as its **step 0, committed before any
broker code**. It is the earliest unbuilt task in env's build order that touches the package (wave 2,
`phase-8-own-environment.md:625-646`; 01, 14, 16 and 28 are built). The package lives on `feat/env`, so
the rework stays there, one package, not two (HA-1).
**Hub task 10** (lane B, first) then writes no validator: it takes env 17's step-0 commit of
`internal/oidcauth` unchanged, through the orchestrator's merge, never a copy. Hub 10 therefore waits for
env 17 step 0, and hub 13 (which waits for 10) inherits that wait.

**`devissuer` emits the `keycloak` profile by default** (hub tasks 11-12, NOT BUILT, lane B): Keycloak
paths (section 2), `iss = <base>/realms/panacea-app-dev`, `typ`, per-audience `aud` by requested scope,
`azp`, `scope`, `oid`/`tid` from `dev-tenant.json`, `roles`, `email`; `offline_access` refresh tokens
with narrowing `scope` on refresh; `kc_idp_hint` accepted and ignored; `login_hint` selects the dev user;
a client-credentials grant for service clients (section 6). `-profile entra` keeps task 11's
Entra-v2 shape for the entra-profile end-to-end row only. `RevokeRefreshTokens(oid)` and
`RequireInteraction(oid)` (task 12) are unchanged in meaning.

