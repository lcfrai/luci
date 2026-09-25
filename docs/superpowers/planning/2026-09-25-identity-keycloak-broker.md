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

