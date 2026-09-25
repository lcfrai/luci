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
5. **Broker.** Keycloak skips its own page (the desktop client's browser-flow override, K1; `kc_idp_hint`
   is sent too but only as a hint), redirects to Entra's tenant-specific
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
   60 seconds before `exp`. The refresh token returned by a narrowed refresh is **never written back**:
   the stored offline token stays the original, all-scope one (section 12, U-4b). The renderer never
   holds any token.

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
| Keycloak's login page (username/password + "Microsoft (KPMG)" button) | **K1's browser-flow override** on `panacea-app-desktop`: flow `panacea app desktop browser`, one step, the Identity Provider Redirector with `defaultProvider = entra`; **no Cookie step and no username/password form** (section 10). Every interactive sign-in therefore goes to Entra, so conditional access always applies and M1/M2 run at every sign-in. `kc_idp_hint=entra` is still sent but is **cosmetic**: a client-supplied parameter carries no trust, and dropping it lands on Entra all the same (OD-30 review-1 I1). |
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

**Out of scope for zero clicks: multi-user hosts** (AVD session hosts, jump boxes, any machine where
another user's process can listen on `127.0.0.1`). With consent off, a flow someone else starts with their
own PKCE verifier completes silently, and PKCE does not stop that. Zero clicks is ruled for a single-user
managed Windows device only. If multi-user hosts are ever supported, a Keycloak client policy requiring
consent for them comes first (a later owner decision; nothing builds it now).

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
| `aud` | string or array, **contains** the service's audience: `panacea-app-hub`, `panacea-app-broker`, `panacea-app-gateway` or `panacea-app-fleet`. May also hold Keycloak's `account`. | contains the service's own, and **no other `panacea-app-*` audience** (section 5) |
| `azp` | `panacea-app-desktop` for a person; a service client id for a service caller (section 6) | in the service's allowlist |
| `oid` | Entra object id, GUID | **required** for a person token; absent on a service token |
| `tid` | Entra tenant id, GUID | required for a person; must be an enrolled tenant (the Hub's org key) |
| `scope` | space-separated; contains the audience scope's name | Keycloak's `scope` replaces Entra's `scp` |
| `roles` | flat array of realm role names; the Hub reads **`hub.org_admin`** | replaces the Entra app role `Hub.OrgAdmin` (hub ADR-H1 `:430`) |
| `email`, `email_verified` | from Entra through Panacea's `email` IdP mapper, syncMode **`FORCE`** (K3), `trustEmail:true`; user-profile `email` is admin-edit only (K4) | the Hub's invite match: **`bindEmail` = `email` only when `email_verified == true` and `azp == panacea-app-desktop`**; otherwise no invite binds. Never `preferred_username`. This replaces the pre-A15 contract rule (`:318-319`) and plan-h's `acct`/`xms_edov` binding (`:194`, `:539`), which fail closed under `keycloak` (ADR-H1 `:800-804`) |
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
  confused-deputy defence, `:2070`, kept). Every validator also **refuses a token carrying a second
  `panacea-app-*` audience** (section 5), so if narrowing ever fails (U-4) sign-in breaks loudly instead
  of the defence lapsing quietly.
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
`Issuer` -> `typ == "Bearer"` -> `aud` contains `Audience` **and contains no other `panacea-app-*`
audience** (`ErrAudience`; Keycloak's `account` is ignored) -> `azp` in `AllowedClients` or `ServiceClients`
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
  and as array; `aud` missing; `aud` = `[panacea-app-hub, panacea-app-broker]` refused `ErrAudience`
  (two audiences); `aud` = `[panacea-app-hub, account]` accepted; `azp` not allowed; person token with no `oid`; `tid` not enrolled
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

## 6. Service callers

**A service that calls the Hub or the broker in its own name is a confidential client with a service
account in the realm, using the client-credentials grant.** This replaces Entra workload identity (the
app-only Entra token with `idtyp=app` and `roles`) **for calls into PanaceaApp services only**. Azure-plane
access (Postgres, Key Vault, storage, AKS) stays Azure workload identity, unchanged (P8-D10; IT request
Part 2).

| Client | Who | Token request | Accepted by |
|---|---|---|---|
| `panacea-app-fleet` | Fleet, team mode (`cmd/fleetd`) | `grant_type=client_credentials`, `scope=panacea-app-hub` or `scope=panacea-app-broker` (one audience per token, as for people) | the Hub and the broker, as a **service client** (`ServiceClients`) |
| `panacea-app-gateway` | the gateway's route source and usage roll-up (phase 6 task 13, phase T task 66) | `grant_type=client_credentials`, `scope=panacea-app-hub` | the Hub only, as a service client |

- **Client settings:** `publicClient:false`, `serviceAccountsEnabled:true`, standard flow off, implicit
  off, direct grants off, no redirect URIs, `clientAuthenticatorType: client-secret`. The audience scopes
  A1-A2 are optional scopes on it. No `oid`/`tid` mappers: a service token carries no person.
- **The secret** lives in the environment's Key Vault and reaches the pod through the Secrets Store CSI
  driver, never an env file, never a laptop. Locally, `devissuer`'s client-credentials grant accepts
  the dev secret from `dev-tenant.json`.
- **A service token's `Claims`**: `Service = true`, `ClientID = azp`, `ObjectID` and `TenantID` empty,
  `Key() == "client/<azp>"` (section 5).

**The delegated-callers amendment keys on `azp`.** Team design `05-design.md:1127` and phase T
`:419-425` said the Hub's `azp` allowlist "gains Fleet's workload identity". It now gains
**`panacea-app-fleet`** in the Hub's `ServiceClients`. The rule is otherwise as written: the Hub refuses
a request with `azp == panacea-app-fleet` unless it names a live 24-hour delegation that the person's own
desktop token created; the actor recorded is the person, with Fleet as the delegate. The broker's second
caller (env 17/18 amendment, phase T task 17) is recognised the same way: `Service` true and
`azp == panacea-app-fleet`, where phase T task 17 said "no `scp`, `azp` = Fleet". Desktop to Fleet stays a
**person** token: audience `panacea-app-fleet`, `azp == panacea-app-desktop`, owner `(tid, oid)` (phase T
task 12), validated by `oidcauth` under `keycloak`.

**The gateway's Hub routes, the one list** (every other file points here; phase T task 66's
`PANACEA_APP_HUB_GATEWAY_CLIENT_ID` is set to `panacea-app-gateway`). The Hub accepts
`Service && azp == panacea-app-gateway` on these read/write routes only, and refuses it on every other:

| Route | Why | Source |
|---|---|---|
| `GET /v1/projects/{projectId}` (the project record; path UNVERIFIED, phase 6 task 13 names it) | the route source (phase 6 task 13's one `GET`) | phase 6 task-13 `:17`, `:26` |
| 74 `GET /v1/projects/{projectId}/team-policy` | team budgets (team mode) | team task-01 `:281`; task-66 `:931` |
| 76 `POST /v1/orgs/{orgId}/usage-rollups` | the daily usage roll-up | team task-01 `:283` |
| 80 `GET /v1/orgs/{orgId}/quotas` | the org quota ceiling (team mode) | team task-66 `:156` |

Routes 74, 76 and 80 exist only once phase T is built; before then the list is the project read alone.
Adding a route to this list is a ruling change.

**The gateway's mint (phase 6) validates its caller under the same profile.** keysmith's
`PANACEA_APP_GATEWAY_CALLER_AUTH` gains a third mode, **`oidc`**, beside `kube` and `bearer-file`, and
becomes a comma list (cloud: `kube,oidc`):
- `kube`: the broker's service account by TokenReview, **unchanged** (phase 6 task 08). The `Owner{TID, OID}`
  the broker sends is the one it derived from the person's `panacea-app-broker` token (P8-D4, section 1).
- `oidc`: a person's own token, audience `panacea-app-gateway`, `azp == panacea-app-desktop`, validated by
  `oidcauth` under `keycloak`. The owner is **the token's `(tid, oid)`**; a request body whose `Owner`
  differs is refused `403`. This is the local-mode laptop minting against the cloud gateway (phase 6 task 15's
  `gatewayclient` sends `TokenSource.Token(ctx, "panacea-app-gateway")`). No service client is accepted.
- LiteLLM's `user_id` stays `"<tid>:<oid>"` (P6-D6), so usage attribution per person is unchanged.

**Leavers.** A person disabled in Entra cannot complete a new broker login, but an offline token already
issued keeps working until it lapses. The desktop client therefore sets
`client.offline.session.max.lifespan = 604800` (7 days), which needs the realm's
`offlineSessionMaxLifespanEnabled: true` (section 10): at most once a week the silent refresh fails and the
zero-click flow re-checks the person against Entra (K1's browser flow has no Cookie step, so every
interactive sign-in goes back to Entra, whatever the realm's SSO session). Fleet's 24-hour delegation lapse
stands. Its daily account-state check (`05-design.md:163-185`) cannot read Entra's state through the realm
(the IdP has `storeToken:false` and no user sync, facts §3); it stays as that design says, flagged in section 12.

**Revocation latency.** Services validate access tokens offline (JWKS, no introspection), so a revoked or
leaver session stays valid at every service until its last access token expires. K1 sets the desktop
client's access-token lifespan to **600 seconds** (Panacea's dev realm default is 1800, facts `:91`), so the
bound is at most 10 minutes after a refresh first fails, **on top of** the 7-day offline cap.

**The realm-admin trust boundary.** Until phase 8, whoever administers Panacea's realm can set any user's
`entra_oid`/`entra_tid`, assign `hub.org_admin`, or impersonate a user, and so become any PanaceaApp
person. PanaceaApp trusts Panacea's realm admins exactly as far as it trusts its own; that ends when phase 8's
realm (our admins only) replaces it. Nothing in PanaceaApp can detect it (section 12, U-15).

## 7. Azure DevOps and GitHub

**Azure DevOps is a personal access token per user. Q5 flips to No.** A realm token cannot be an Azure
DevOps token: ADO accepts only Entra tokens for its own resource (`499b84ac-1321-427f-aa17-267ca6975798`),
and the broker's Entra login is for `openid profile email` only (`setup-keycloak.sh:508`) with
`storeToken:false` (`:485`).

- The user creates a PAT in Azure DevOps with scope **Code (Read & write)** and pastes it once into the
  desktop's connection panel; it is stored in Windows Credential Manager through `credstore` (OD-6's store,
  OD-4). This is hub task 29's existing PAT path, now the only ADO path.
- **Removed from the plans:** `AzureDevOpsScope` and the `DevOps` credential with Source `"entra"` (hub
  task 31 part `:31-33`, `:184-241`), `PANACEA_APP_ENTRA_DEVOPS` (hub task 32 part `:84`), the credstore
  source `"entra"` for DevOps (Hub contract §9 `:1269`), and the ADO `user_impersonation` permission (IT
  request item 2, `2026-09-24-it-request-entra.md:19-27`).
- **Optional later IT ask, not in any task:** a desktop public-client registration with delegated
  consent for Azure DevOps `user_impersonation`, which would let the control plane run a second, direct
  Entra PKCE for the ADO resource only. It is raised only if PAT expiry becomes a real support cost.

**GitHub is unaffected**: its own token in the same store (OD-4; hub task 29), as before.

## 8. Phase 7: Panacea's API

**Verified from Panacea's code: a realm access token does NOT authenticate to Panacea's API today.**
- REST routes: `Auth` honours a bearer only if it equals `BACKEND_SECRET`
  (`services/panacea-api/middleware/auth.go:156-160`); any other non-`pnc_` bearer is `401`
  (`:245-251`).
- Conduit and MCP, the direct-bearer surfaces: one JWKS, one issuer, one audience at a time
  (`middleware/jwks.go:149-151`), deployed as the Backstage portal's keys, issuer `https://<domain>/api/auth`,
  audience `backstage` (`deploy/portal/deploy-portal.yaml:605-621`). The realm's keys, issuer and
  audience all differ. F3 (`deploy/f3-entra-cutover.sh`) re-points that one verifier at **Entra**, not
  Keycloak, so it does not help either.

**Consequence for phase 7's onboarding: none. 7P-1 stands.** Phase 7 authenticates to Panacea with the
user's own `pnc_` key, pasted once into the ConnectPanel and stored at `PanaceaApp/panacea/<host>`
(phase 7 plan `:53`, `:82-84`, `:167-192`; phase 7 task 03's client, BUILT, sends `pnc_`). 7P-1's reasons 1
and 3 hold as written; reasons 2 and 4 now read "a realm token" for "an Entra token" and reach the same
answer. Phase 7 task 11's grantee resolution to a Keycloak user is already consistent with OD-30.
**Not asked of Panacea now:** a second trusted issuer (the realm) with an audience `panacea-api`. If
Panacea ever adds it, the desktop requests one more audience scope and the paste step goes; until then
nothing in phase 7 waits on it.

## 9. Phase 8: our own Keycloak

**Phase 8 runs its own Keycloak, modelled on Panacea's, with the realm as code.** Realm `panacea-app`
at `https://auth.<our domain>/realms/panacea-app`. The move is one variable
(`PANACEA_APP_OIDC_ISSUER`); the person key `(tid, oid)` is the same in both realms, so no Hub row, owner
label or LiteLLM user changes.

**What is copied from Panacea's pattern (as a pattern, no source, OD-13), and what is not:**
- **The same Entra IdP on the SAME registration** (client id `37f8bd70-...`), alias `entra`, tenant-specific
  endpoints, `clientAuthMethod: client_secret_post`, `tid` claim filter, `trustEmail:true`,
  `storeToken:false`, the email/name/username mappers, and a first-broker-login flow of
  `idp-create-user-if-unique` or `idp-auto-link` with no review-profile step (Panacea
  `setup-keycloak.sh:375-635`). Plus section 4's M1, M2, UP, P1-P3, A1-A4, the "Pass login_hint" switch,
  section 10's K1 browser-flow override, K3's `email` mapper on `FORCE`, K4's user-profile permissions and
  `unmanagedAttributePolicy`, K6's rule for person clients, and section 6's service clients.
- **One Entra change, the only Entra ask OD-30 leaves:** IT adds **one redirect URI**,
  `https://auth.<our domain>/realms/panacea-app/broker/entra/endpoint`, to the registration, and **a
  second client secret** for our Keycloak (so either side rotates without breaking the other), stored in
  our Key Vault. It joins IT request Part 2 and is sent when phase 8 provisions.
- **Not copied: Panacea's outage class** (ADR-H1 §6.1). Two replicas with Keycloak's persistent user
  sessions in Postgres, not one replica on `--cache=local` (`deploy/main.tf:928`, `:954-957`); the image
  pinned by digest to the current Keycloak 26 release, not the chart default.
- **Realm as code that converges, not import-once:** `deploy/keycloak/realm/panacea-app.json` in
  keycloak-config-cli's format (Apache-2.0), applied in-cluster by a Job on every deploy, like Panacea's
  unconditional Phase 12 run (`deploy/cloud-shell-deploy.sh:5942`, `:2453`). The admin API is never
  exposed beyond the cluster. `--import-realm` is not used: it skips a realm that exists, so a changed file
  would never land ("exists" is not "correct").
- **Switching issuers:** the credstore target for the refresh token names the issuer (section 11, hub
  task 28), so a token from Panacea's realm is never offered to ours; the first launch after the switch
  finds none, and the zero-click flow signs the person in to the new realm.

**Which tasks carry it** (earliest unbuilt tasks in env's build order that touch each part; none is new):

| Part | Task | Wave | Change |
|---|---|---|---|
| The realm file and its test (every client, scope, mapper and IdP setting of sections 4, 6, 10, asserted by a Go test that parses the JSON) | **env 09** `stacks/identity` | 2 | Its Entra registrations (*Desktop*, *Hub API*, *Broker API*, HA-3) are **deleted**; its Entra admin groups (cluster admins, Postgres admins) **stay**; the "product users" group is deleted (the realm's `tid` filter admits the tenant) |
| Keycloak's namespace, Pod Security and network policy | **env 11** `deploy/charts/platform` | 2 | add the `panacea-identity` namespace (env task-11 `:43`, `:96`; task-29 `:215`) |
| Egress from Keycloak to `login.microsoftonline.com`; Hub and broker to the realm's JWKS | **env 07** `stacks/foundation` | 4 | the egress list gains the realm host (facts-app-identity §2d) |
| Keycloak's database and role | **env 08** Postgres | 5 | one database `keycloak` (auth method: section 12, U-9) |
| The Keycloak release (two replicas), the realm Job, per-environment values | **env 29** `deploy/charts/broker` and `hub` | 5 | add `deploy/charts/keycloak` beside them |

## 10. The Panacea asks, exact

For Panacea's own update of realm `panacea`, dev estate first (`auth.dev.panacea.kpmg`). Where: a new block
in `deploy/portal/setup-keycloak.sh` beside `panacea-oauth2-proxy` (`:308-373`), in the same
create-then-PUT-on-409 shape (`:332-361`); it ships through Phase 12's unconditional run
(`deploy/cloud-shell-deploy.sh:5942`) by Panacea's ADO pipeline. **Nothing in PanaceaApp blocks on it**:
`devissuer` runs the same flow, and the Panacea issuer is only configuration.

**K1. Public client `panacea-app-desktop`** (name "PanaceaApp Desktop"):

| Field | Value |
|---|---|
| `publicClient` | `true` (no secret) |
| `standardFlowEnabled` | `true`; `implicitFlowEnabled`, `directAccessGrantsEnabled`, `serviceAccountsEnabled`: `false`; OAuth device grant: off |
| `attributes."pkce.code.challenge.method"` | `S256` (required) |
| `redirectUris` | `["http://127.0.0.1/callback"]`: the one path the listener serves, any loopback port (RFC 8252 §7.3); never `localhost` (facts §8); never a `/*` path. U-3 settles the port form |
| `webOrigins` | `[]` (none: the control plane is not a browser, so no CORS) |
| `consentRequired` | `false` (zero clicks; single-user devices only, section 3 "Out of scope") |
| `authenticationFlowBindingOverrides.browser` | the id of flow **`panacea app desktop browser`** (below). No `direct_grant` override: direct grants are off |
| `attributes."client.offline.session.max.lifespan"` | `604800` (7 days, section 6) |
| `attributes."access.token.lifespan"` | `600` (10 minutes): bounds how long a revoked or leaver session stays valid at the offline validators (section 6) |
| default client scopes | the realm's defaults, unchanged |
| optional client scopes | `offline_access`, `panacea-app-hub`, `panacea-app-broker`, `panacea-app-gateway`, `panacea-app-fleet`. **Not** `groups` |
| dedicated mappers | P1 `oid`, P2 `tid`, P3 `roles` (section 4) |

**K1 flow `panacea app desktop browser`** (a new top-level flow, `providerId: basic-flow`; Panacea's
realm `browser` flow is not touched): **one** execution, `identity-provider-redirector`, requirement
`REQUIRED`, config `defaultProvider = entra`. Nothing else: no `auth-cookie`, no
`auth-username-password-form`, no OTP form. The client-supplied `kc_idp_hint` therefore decides
nothing (section 3). Refresh and offline grants do not run a browser flow, so silent launches are
unaffected. Env 09's realm test asserts the same flow and binding for phase 8's realm.

**K2. Four client scopes** `panacea-app-hub`, `panacea-app-broker`, `panacea-app-gateway`,
`panacea-app-fleet`: protocol `openid-connect`, `include.in.token.scope: true`, one `oidc-audience-mapper`
each with `included.custom.audience` = the scope's name, access token only (A1-A4).

**K3. IdP `entra` changes**, nothing else about it touched:
- `config.loginHint: "true"` ("Pass login_hint"). Harmless to Panacea's clients: it forwards a hint only
  when a client sends one.
- Mappers M1 (`oid` -> `entra_oid`) and M2 (`tid` -> `entra_tid`), `oidc-user-attribute-idp-mapper`,
  syncMode `FORCE`. The script's expected mapper count (`:635`) goes from 6 to 8.
- Panacea's existing `email` IdP mapper (facts `:47`, `:57-58`, today INHERIT, so `IMPORT`) becomes
  syncMode **`FORCE`**, so Entra rewrites `email` at every login and a stale or self-edited value never
  survives one (the Hub's invite match relies on it, section 4).
- **Unchanged and relied on:** `prompt` unset, `pkceEnabled:false`, `syncMode: IMPORT` on the IdP,
  `trustEmail:true`, the `tid` filter, and `firstBrokerLoginFlowAlias = "panacea first broker login"`.

**Order: K4 lands in the same script block as K3, and before it**, and the script re-asserts K4's user
profile on every run (the create-then-PUT shape). If K3's FORCE mappers ran first, or a later run PUT a
user profile without the two declarations, `entra_oid` would be unmanaged, and a user could then edit
it (a forged `oid`).

**K4. Realm settings:**
- User profile declares `entra_oid` and `entra_tid`: permissions view `["admin"]`, edit `["admin"]`,
  not required, no validators beyond length 36.
- User profile `email`: permissions edit `["admin"]`, view `["admin","user"]`, so a user cannot change
  it in the account console (Keycloak's default lets the user edit it; the live realm is UNVERIFIED).
- User profile `unmanagedAttributePolicy` is stated, and is **not `ENABLED`** (absent, or
  `ADMIN_VIEW`/`ADMIN_EDIT`), so no undeclared attribute is ever user-editable.
- `offlineSessionMaxLifespanEnabled: true` with `offlineSessionMaxLifespan: 5184000` (60 days, Keycloak's
  own value once enabled), so K1's 7-day client override takes effect. It caps any other client's offline
  sessions at 60 days (UNVERIFIED that Panacea has any; section 12).
- Realm role `hub.org_admin`, assigned by the realm admin to the owner's user.

**K5. Later, only if team mode or the gateway runs against Panacea's realm before phase 8's Keycloak
exists:** confidential clients `panacea-app-fleet` and `panacea-app-gateway` exactly as section 6. If
phase 8's Keycloak comes first, they are born in our realm (section 9) and K5 is never asked.

**K6. Every realm client that carries a person** (the desktop now; the Hub pages client and approved
third-party app clients, phase W, when they are asked for) gets **P1 and P2** (`oid`, `tid`) as dedicated
mappers and **only the audience scopes it needs** as optional scopes, or its tokens carry no `oid` and
every validator refuses them. The Hub pages client is **confidential** (`panacea-app-hub-pages`,
`publicClient:false`, standard flow on, redirect URIs = the Hub pages' own callback, no service account,
optional scope `panacea-app-hub` only, K1's browser-flow override). Its exact redirect URI and the
third-party clients are phase W's to name (findings-work); the rule above is this ruling's.

**Check after Panacea applies K1-K4** (owner, about 10 minutes): set `PANACEA_APP_OIDC_ISSUER` to the dev
realm, launch PanaceaApp signed out, and confirm:
1. The browser tab reaches "Signed in. You can return to PanaceaApp." with no click; then
   `GET /v1/auth/status` shows a GUID `oid` and `tid`.
2. **Drop `kc_idp_hint` by hand** (paste the authorize URL without it): it still lands on Entra, and
   no Keycloak username/password page appears (K1's flow override, I1).
3. **As the user, POST a changed `entra_oid`** (and a changed `email`) to
   `/realms/panacea/account`: both are refused (K4).

## 11. The per-plan change list

HOW: **edit** = the named file is edited to this ruling now, before its task is built; **fold into NN** =
the change is made by task NN when it is built; **findings** = a findings file for that plan's writers
(`.superpowers/sdd/identity/findings-team.md`, `findings-work.md`), because those plans are still being
written; **none** = verified to need no change. Built code is never edited outside a task.

| Tree | File | Task | Built | Change | How |
|---|---|---|---|---|---|
| main | `CLAUDE.md:21` | — | doc | "signs in with Entra" -> "through a Keycloak realm brokering Entra, auth-code + PKCE, zero clicks (OD-30)" | edit |
| main | `docs/PROGRAM.md:18`, `:59`, `:95` | — | doc | identity row back to the broker chain; delete "a real Entra app registration" | edit |
| main | `owner-decisions.md:186-188` | — | doc | the PROGRAM.md bullet: "the Hub ADR decides" -> "OD-30 decided: a broker" | edit |
| main | `2026-09-24-it-request-entra.md` | — | doc | Part 1 (`:1-54`) withdrawn, nothing asked for sign-in; Q5 (`:104`) flipped; Part 2 gains section 9's redirect URI and second secret; `:3-4` corrected (devissuer not built) | edit |
| hub | ADR-H1 §5.2, §6.1, §9, Q5, `:430`, `:678-679` | 44a | doc | sections 1-4, 7; §6.1 reversed; §9's IT list replaced by section 10 | fold into 44a |
| hub | Hub contract §6.1-§6.3, §6.5, §9, §10 (`orgs`/`users` keys unchanged), §12, T2 | 44a | doc | env vars (section 2); audience scopes for `Hub.Access`; `oidcauth`; §6.5 deleted; ADO PAT | fold into 44a |
| hub | `hubapi/codes.go:57-67` | 01 | BUILT | none: `ErrType` maps to `ReasonMalformed`; `ReasonVersion` is unused under `keycloak` | none |
| hub | `hubapi/types.go:104`, `codes.go:21` | 01, 03 | BUILT | none: `tid` arrives as a claim (P2) | none |
| hub | `hubclient/hubclient.go:18,23,137` | 04 | BUILT | comments name `entraauth` -> `signin` | fold into 31 |
| hub | parts `task-10.md` | 10 | no | writes no validator; takes env 17 step 0's `internal/oidcauth` | edit |
| hub | parts `task-11.md`, `task-12.md` | 11, 12 | no | devissuer: Keycloak paths and `keycloak` profile by default, client-credentials grant, scope narrowing (section 5) | edit |
| hub | parts `task-13.md` | 13 | no | `oidcauth` under `keycloak`, audience `panacea-app-hub`, `ServiceClients` Fleet + gateway (section 6); rate-limit keys `tid/oid` unchanged | edit |
| hub | parts `task-28.md` | 28 | no | refresh-token target `PanaceaApp/oidc/<issuer host>/<realm>/refresh` replaces `EntraRefreshPrefix`/`EntraAccountTarget` | edit |
| hub | parts `task-29.md` | 29 | no | ADO: PAT is the only path; the Entra ADO branch deleted | edit |
| hub | parts `task-30.md`, `task-31.md` | 30, 31 | no | package `entraauth` -> `signin`; issuer from discovery; `kc_idp_hint`, `login_hint` (UPN), `offline_access`, per-audience refresh; `ErrEntraUnavailable` -> `ErrIssuerUnavailable`; `AzureDevOpsScope` deleted | edit |
| hub | parts `task-32.md` | 32 | no | `PANACEA_APP_OIDC_*`; `TestTheAuthorityMustNameATenant` -> `TestTheIssuerIsARealmURL`; `PANACEA_APP_ENTRA_DEVOPS` deleted | edit |
| hub | parts `task-33.md`, `task-07b.md`, `task-09.md` | 33, 07b, 09 | no | wording "Entra" -> "the issuer"; `PANACEA_APP_HUB_SCOPE` -> `PANACEA_APP_HUB_AUDIENCE=panacea-app-hub`; `TestResolvePrincipal` unchanged | edit |
| hub | parts `task-40.md`, `task-41.md` | 40, 41 | no | no sign-in click at launch; the loopback page text; foregrounding (section 3) | edit |
| hub | parts `task-43.md` | 43 | no | real-tenant checklist becomes section 10's check against Panacea's dev realm | edit |
| env | `control-plane/internal/entra/*` | 16 | BUILT | rename to `internal/oidcauth`, add the profile (section 5) | fold into 17 (step 0) |
| env | `internal/sandbox/manager.go:138-141` | 14 | BUILT | none: `(tid, oid)` are still Entra's ids | none |
| env | phase 8 plan `:123` OQ-4, `:134` P8-D4, `:250-264` HA-1/HA-3, `:484`, `:562`, `:584` | — | doc | sections 1, 4, 5, 9 | edit |
| env | parts `task-09.md` | 09 | no | rewritten per section 9 | edit |
| env | parts `task-17.md` | 17 | no | step 0; `PANACEA_APP_OIDC_ISSUER`/`_PROFILE`, `PANACEA_APP_BROKER_AUDIENCE=panacea-app-broker` replace `_ENTRA_*` (`:33-36`); `ServiceClients` = Fleet | edit |
| env | parts `task-18.md`, `task-22.md`, `task-23.md` | 18, 22, 23 | no | "Entra token" -> "realm token"; task 22's scope -> `panacea-app-broker` | edit |
| env | parts `task-07.md`, `task-08.md`, `task-11.md`, `task-29.md` | 07, 08, 11, 29 | no | section 9's rows | edit |
| phase-6 | parts `task-08.md` | 08 | no | `CALLER_AUTH` comma list, new `oidc` mode (section 6) | edit |
| phase-6 | parts `task-13.md` | 13 | no | route source: client credentials as `panacea-app-gateway` at the realm, not an Entra federated assertion | edit |
| phase-6 | parts `task-15.md`; `task-03.md` | 15; 03 | no | 15 sends the person's `panacea-app-gateway` token in `oidc` mode; 03's `Owner{TID, OID}` unchanged | edit; none |
| phase-7 | plan `:140`, `:167-192`, `:205-212` | — | doc | "Entra token" -> "realm token"; 7P-1 stands (section 8) | edit |
| phase-4 | parts `task-23.md:29`, `:129` | 23 | no | none: devissuer stays the local issuer | none |
| team | `05-design.md` `:120`, `:161-162`, `:189-191`, `:853`, `:1127`; phase T plan and parts 01, 05, 12, 17, 21, 66 | T* | no | section 6: Fleet and gateway service clients, `azp` keys, audience `panacea-app-fleet` | findings |
| work | phase W plan `:127`, `:254`, `:257`, `:568`, `:581`; scopes 141-160, 161-180, 181-187 | W* | no | the MCP server's `authorization_servers` = the realm; Hub pages sign in through the realm as a confidential client (K6); third-party apps are realm clients under K6's rule; `groups` stays absent (section 4) | findings |

## 12. UNVERIFIED

Each item names who settles it. None changes a decision above; each says what happens if it is false.

| # | Not verified | Settled by | If false |
|---|---|---|---|
| U-1 | That OD-30's "Panacea Platform API" is the registration Panacea's IdP uses (`37f8bd70-...`, named `AU-PROD-Panacea-RITM6316333`; facts §10) | the owner, one look | the ruling holds: it keys on that client id, not the name |
| U-2 | Panacea's **live** realm: no Identity Provider Redirector default, no required actions, the IdP bound to `panacea first broker login`, IdP settings as scripted (facts §3, §4) | Panacea's team when applying K1-K4 | a live difference is fixed in Panacea's script, not worked around in PanaceaApp |
| U-3 | The live Keycloak is 25.0.0 (chart default, pod not inspected); loopback any-port, `kc_idp_hint` and the `loginHint` switch in exactly 25.0.0 (facts §1, §8) | section 10's check | if a loopback port is refused, K1 lists `http://127.0.0.1:*/*`-style entries per Keycloak's own doc for that version |
| U-4 | Keycloak honours a narrowing `scope` on the refresh-token grant, so each access token has one audience | hub task 31 against devissuer, then task 43 against the dev realm | tokens carry all four audiences and **every validator refuses them** (`ErrAudience`, section 5): a loud failure at hub task 43, never quiet degradation. The owner then decides (per-audience clients, or phase 8's own Keycloak) |
| U-4b | The refresh token Keycloak returns from a **narrowed** refresh keeps the **original** scope (all four audience scopes plus `offline_access`) | hub task 43's dev-realm check: Hub -> broker -> gateway -> Hub on one stored token | the narrowed refresh token would make the next `Token(hub)` fail `invalid_scope`. So task 31 **never writes back a refresh token returned by a narrowed grant**; it keeps the original offline token (safe: `revokeRefreshToken` is false, facts `:94-96`). devissuer (task 12) pins that keep-original behaviour. This rule holds whether U-4b is true or false |
| U-5 | Offline sessions survive a pod restart of Panacea's one-replica, `--cache=local` Keycloak; the realm's default roles include `offline_access` | section 10's check, then a pod restart | a restart costs one zero-click re-sign-in; nothing else changes |
| U-6 | Zero clicks on KPMG devices: the PRT browser SSO (Edge; Chrome with Windows Accounts), conditional-access behaviour, and existing consent for `openid profile email` on the registration | section 10's check, by the owner | the prompts are Entra's and correct (section 3); none is worked around |
| U-7 | `GetUserNameExW(NameUserPrincipal)` returns the Entra UPN on KPMG's (hybrid or Entra-joined) devices | hub task 30, one run on the owner's laptop | `login_hint` is omitted; Entra relies on the browser session |
| U-8 | Keycloak 25 drops an IdP-mapped attribute that the user profile does not declare | not needed: UP declares it either way | — |
| U-9 | Keycloak can reach phase 8's **Entra-only** Postgres by workload identity (Azure's `azure-identity-extensions` JDBC plugin as a provider jar); Keycloak has no native Entra auth to Postgres | env task 08 | env 08's implementer stops and reports; no password role is invented without the owner |
| U-10 | Whether any Panacea client holds offline sessions that K4's 60-day cap would shorten | Panacea's team | they gain a 60-day cap; Panacea's choice to accept |
| U-11 | Tauri `setFocus` against Windows' foreground lock | hub task 40 | the taskbar flash is the whole behaviour |
| U-12 | Fleet's daily account-state check (`05-design.md:163-185`) has no way to see Entra disablement through the realm | the team plan's writers (findings) | the 7-day offline cap (section 6) is the leaver bound |
| U-13 | Keycloak token exchange is still preview, and keycloak-config-cli supports the Keycloak 26 release pinned in phase 8 (both from knowledge, not fetched) | env tasks 09 and 29 | WAM stays out (section 3 has two other reasons); the realm Job's tool is re-chosen by env 09 from Apache/MIT tools only |
| U-15 | Panacea's realm admins can set `entra_oid`, assign `hub.org_admin` or impersonate, and so become any PanaceaApp person (section 6); and the 600 s access-token override takes effect on Keycloak 25.0.0 | accepted by this ruling until phase 8; the override by section 10's check (decode one token's `exp - iat`) | the risk stands until phase 8's own realm; if the override is ignored, the realm's 1800 s applies and revocation latency is 30 minutes |
| U-14 | Line numbers for hub parts 07a, 15, 17, 21, 27, 34, 36, 37, 39, 40, 41, 42, and the phase 6, team and work rows of section 11, are from `facts-app-identity.md`, not re-read here | each task's implementer re-verifies its part's citations (OD-29) | the edit lands on the right text, not the cited line |

