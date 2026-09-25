# Request to KPMG IT — Entra registrations for PanaceaApp (draft 2026-09-24, amended by OD-30 2026-09-25)

**Status (2026-09-25, OD-30): nothing is sent to IT for sign-in.** Users sign in through a
Keycloak realm that brokers Entra on **Panacea's existing registration**, so Part 1 below is
withdrawn. Part 2 (phase 8's Azure plane) is still sent, when phase 8 provisions. Ruling:
`docs/superpowers/planning/2026-09-25-identity-keycloak-broker.md`.

**Nothing is blocked on this file.** The Hub is planned against a local OIDC issuer, `devissuer`
(hub tasks 11-12, **not built yet**), which runs the identical Keycloak-shaped flow. Source for the
original draft: ADR-H1 and the Hub contract on `feat/hub`
(`docs/superpowers/adr/ADR-H1-hub-architecture.md`,
`docs/superpowers/contracts/2026-09-23-hub-contract.md`) and phase 8's plan.

---

## Part 1 — sign-in registrations: Not needed since OD-30

**Withdrawn 2026-09-25. Do not send.** Items 1-7 of the earlier draft (the tenant id; the
*PanaceaApp Desktop*, *PanaceaApp Hub API* and *PanaceaApp Sandbox Broker* registrations; the
"PanaceaApp Users" and "PanaceaApp Org Admins" groups and the `Hub.OrgAdmin` app role; the
conditional-access question; Azure DevOps sign-in) are all withdrawn. The text is in git history.

Why no IT ticket (ruling section 1): the desktop signs in to a Keycloak realm, and that realm
brokers to Entra through the registration Panacea's Keycloak already uses (client id beginning
`37f8bd70-`). Entra only ever sees Keycloak's broker callback, which is already registered. The
desktop's loopback redirect is registered on a new **Keycloak client**, never on Entra, and that
client is added by Panacea's own realm update (ruling section 10), not by IT. Org admin is the
realm role `hub.org_admin`, assigned by the realm's admin. Azure DevOps is a personal access token
(Q5 below).

**Kept: one optional later ask, not in any task** (ruling section 7). Raise it only if ADO
personal-access-token expiry becomes a real support cost:

> Could you please create a single-tenant **public client** registration, "PanaceaApp Desktop",
> platform "Mobile and desktop applications", redirect URI `http://127.0.0.1` (loopback, any
> port), no secret, with delegated permission **Azure DevOps `user_impersonation`** and tenant-wide
> admin consent? It lets the desktop get an Azure DevOps token for the signed-in user by
> authorization code + PKCE, so users need no ADO personal access token.

---

## Part 2 — only when the new environment is provisioned (phase 8)

Not needed yet. The Terraform is written, formatted and validated, and nothing is planned or
applied until this exists. Source: `feat/env`,
`docs/superpowers/planning/env/survey-own-environment.md` §7 and the phase 8 plan's P8-D decisions.

**Before the first `terraform plan`:**
1. **A dedicated Azure subscription**, its tenant id, and the region. Recommended: **Australia East**
   (three availability zones).
2. **vCPU quota** in that region for a nested-virtualisation Gen2 SKU (`Standard_D8s_v5`), which Kata
   pod sandboxing needs, plus the system and apps pools.
3. **A deploy identity** with role-assignment rights, scoped by an ABAC condition to a role allowlist
   (the simpler alternative is Owner on the subscription). Four pipeline identities split by duty:
   plan/read-only, infra, cluster, push. The owner grants each one's roles.
4. **An Azure DevOps project** with a workload-identity-federation service connection to the
   subscription, paid parallel jobs, and a VNet-injectable agent pool (Managed DevOps Pool), or the
   right to create one.
5. **A Terraform state location** (storage account with versioning, soft delete and Entra-only
   access), or the right to create it.

**Before the first apply:**
1. **DNS**: a name, its delegation from the parent zone, and the certificate policy. Is a public
   ACME CA allowed, or must certificates come from a KPMG CA?
2. **Network posture**: internet-facing with a KPMG IP allowlist, or corporate-only through peering
   or ExpressRoute? Plus a VNet address space that does not overlap the corporate network.
3. **Data classification** (for example, PROTECTED or not). It decides sandbox egress: NAT with
   network policy, or an FQDN-allowlist Azure Firewall at roughly A$1.5–2k a month more. It also
   decides customer-managed keys and Defender.
4. **A monthly budget**, with alert recipients.
5. **Entra groups**: cluster admins (AKS Entra RBAC), and a Postgres admin group. The Hub's
   database roles are created once by a human member of that group, never by a pipeline.
6. **Our own Keycloak on the existing registration** (OD-30, ruling section 9; the only Entra ask
   OD-30 leaves). On the registration Panacea's Keycloak uses (client id beginning `37f8bd70-`):
   - add **one redirect URI**,
     `https://auth.<our domain>/realms/panacea-app/broker/entra/endpoint`;
   - issue **a second client secret** for our Keycloak, so either side rotates without breaking
     the other. It is stored in our Key Vault, never sent by email.

**Before real users:**
- Confirmation that our org gateway may reach the direct Anthropic API (Global) and Amazon
  Bedrock in Sydney (AU) from Azure egress IPs, on org-level accounts with zero data retention
  (OD-19). No personal subscription is used; sandboxes hold only a scoped per-session gateway
  token, never an org key.
- The Azure Policy assignments on the subscription, and exemptions if they block Kata SKUs, public
  IPs or the chosen network plugin.
- Who is on call, and the log-retention period ISM requires.

## Owner questions the Hub builds the recommended answer for (OD-1)

These are **accepted by default**. Tell the orchestrator if you disagree.
- **Q1**: only a *complete* brief revision may be approved against or pinned. At Ship, the Hub pins
  the latest complete revision, or records "no complete revision".
- **Q2**: erasure removes a brief's text but keeps key, digest, author and time. Pins render
  "erased on <date>", not "missing".
- **Q3**: a designer may approve their own design in v1.
- **Q4**: teammates join by an email invite within the same tenant (no Graph people picker).
- **Q5** (flipped by OD-30, ruling section 7): Azure DevOps uses a **personal access token** per
  user, scope Code (Read & write), pasted once into the desktop's connection panel and kept in
  Windows Credential Manager. A realm token cannot be an Azure DevOps token. *Was: "the same Entra
  sign-in, so no ADO PAT is needed".*
- **Q6**: while the Hub is unreachable, writes are refused, not queued.
- **Q7**: the Hub holds no canvas documents in v1 (approved snapshots only).
