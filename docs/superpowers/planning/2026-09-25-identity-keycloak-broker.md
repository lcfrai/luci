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

