# Request to KPMG IT — Entra registrations for PanaceaApp (draft, 2026-09-24)

**Status: DRAFT for the owner to send. Nothing is blocked on it.** The Hub is built and tested
against a local OIDC issuer (`devissuer`) that runs the identical PKCE flow. These registrations are
needed only to sign in with real KPMG identities. Source: ADR-H1 and the Hub contract on
`feat/hub` (`docs/superpowers/adr/ADR-H1-hub-architecture.md`,
`docs/superpowers/contracts/2026-09-23-hub-contract.md`); phase 8's plan adds the broker item.

---

**Subject: Entra app registrations for PanaceaApp (internal developer platform pilot)**

Hi team,

We are piloting PanaceaApp, a desktop cockpit for coding agents that signs users in with their
own KPMG identity. Could you please create the following in the **KPMG Australia** tenant?

1. **Our tenant id** (GUID), so we can pin the issuer.
2. **App registration: "PanaceaApp Desktop"**
   - Single tenant; **public client**; platform "Mobile and desktop applications".
   - Redirect URIs: `http://localhost` and `http://127.0.0.1` (the second may need the manifest
     editor).
   - No secret, no certificate. "Allow public client flows": **No** (we use authorization code +
     PKCE with a loopback redirect, not device code or ROPC).
   - Delegated permissions: `openid`, `profile`, `offline_access`, `email`,
     `api://<hub-api-id>/Hub.Access`, `api://<broker-id>/Sandboxes.Access`, and (see item 7)
     Azure DevOps `user_impersonation`.
   - Tenant-wide admin consent for all of the above.
3. **App registration: "PanaceaApp Hub API"**
   - Single tenant. Application ID URI `api://<its client id>`.
   - Delegated scope `Hub.Access`, with "PanaceaApp Desktop" pre-authorised.
   - `requestedAccessTokenVersion: 2`. App role `Hub.OrgAdmin`. Optional access-token claim
     `email`. No secret, no certificate.
   - On its enterprise application: **assignment required = Yes**, assigned to group
     "PanaceaApp Users".
4. **App registration: "PanaceaApp Sandbox Broker"**: scope `Sandboxes.Access`, with the desktop
   pre-authorised.
5. **Security groups**:
   - "PanaceaApp Users": the pilot team.
   - "PanaceaApp Org Admins": assigned `Hub.OrgAdmin`.
   - Plus a Postgres admin group for the environment, when that environment is provisioned.
6. **Conditional access**: please confirm that a public desktop client with a loopback redirect is
   allowed, what device state it requires (compliant or hybrid-joined), and any sign-in-frequency
   interval.
7. **Azure DevOps**: may the desktop use the same Entra sign-in for Azure DevOps
   (`user_impersonation`), so users need no ADO personal access token?

No client secrets are requested anywhere. Tokens stay on the user's machine in Windows Credential
Manager. Access tokens are held in memory only.

Thanks,
Dylan

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

**Before real users:**
- Confirmation that sandboxes may reach Anthropic and the other model providers from Azure egress
  IPs, using users' own subscriptions.
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
- **Q5**: Azure DevOps uses the same Entra sign-in (item 7 above), so no ADO PAT is needed.
- **Q6**: while the Hub is unreachable, writes are refused, not queued.
- **Q7**: the Hub holds no canvas documents in v1 (approved snapshots only).
