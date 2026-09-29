# 06 — Deploy and release provenance: what to adopt instead of building phase 7

**Status:** in progress, 2026-09-30. Research only: nothing was installed or run.
**Capability:** shipping the result — a preview environment per branch, promotion, rollback, and a
release that carries its provenance (the brief it answers, the evidence).
**Replaces (candidate):** phase 7, the Deploy connector, 20 tasks
(`.claude/worktrees/phase-7/docs/superpowers/plans/2026-09-23-phase-7-deploy.md` §8).

---

## 0. The answer

*(written last)*

## 1. What phase 7 has to do, restated as a need

| # | Need | Where it comes from |
|---|---|---|
| N1 | **Ship** a branch from the cockpit to a running URL: a preview environment per branch | phase 7 plan §1; `CLAUDE.md` "nothing to a shipped application" |
| N2 | **Promote** a release to production, gated by the scans its data classification requires | `OD-24` ("It will also determine what security scans are needed from a deployment stand point") |
| N3 | **Roll back** to an earlier release; **rebuild** a release's exact commit | phase 7 plan §1, Q-P7-1 |
| N4 | A **release that carries its provenance**: the brief revision it answers, the sessions that wrote it, and the evidence (build, scans, signature, SBOM), shown in the cockpit and exportable to an assessor | `CLAUDE.md` product spine; `OD-3` |
| N5 | Azure, our environment first, the customer's own cloud later, an AU-only option, Entra through Keycloak, permissive licences only in anything we ship | `OD-5`, `OD-18`, `OD-24`, `OD-30`, `OD-32` |
| N6 | Stay behind the `deploy.Deployer` seam so the Panacea lift is a swap | `OD-13`; plan §2 "The swap" |

**The plan as written.** 20 tasks, **about 51 engineer-hours, "roughly 7 working days for one
engineer, 4 for two"** (plan §8, line 666). It is already a thin client: it adds no Panacea
endpoint and no new Go module. It uses 17 `panacea-api` routes and **deliberately leaves out
`P/promote`** (plan line 480), so N2 is not in it today. Four of the 20 tasks are built.

**What that means for this study.** Phase 7 is small because the deploy product already exists:
it is Panacea. The question is not "which platform replaces a 20-task build" but **"what else
would shorten the remaining 16 tasks, close N2 and N4, and give us a deploy target a customer can
host"**.

## 2. What already exists: Panacea is the deploy platform

Read from Panacea's source at `efc637879` (2026-09-30), read-only. Nothing was run.

| Need | What Panacea already has | Where |
|---|---|---|
| N1 preview per branch | per-branch environments, per-branch auto-deploy, access posture (SSO, IP list, password, bypass tokens), URL prefix | plan §4 rows 4, 14-17 |
| N1 ship | `POST P/deploy` starts a Tekton pipeline with scans; cancel | plan §4 rows 11-12 |
| N2 promote | `POST /api/v1/projects/{name}/promote` and `POST …/envs/{env}/stage/promote` | `routes/routes.go:1027`, `:2818` |
| N2 gate | a posture-policy engine: platform baseline plus team/project tightening, per-check `block` or `ack`, fail-closed, gating **prod-deploy** | `STATUS.md:128-135`; `handlers/security_policy.go` |
| N3 rollback | `POST P/deployments/{id}/rollback`, with warmth and cold-start seconds | plan §4 row 13 |
| N4 evidence | **a per-release evidence document**: release identity (digest, commit, env, PipelineRun), the recorded scan verdicts for *that* release, signature and SBOM references "honestly absent when they did not" exist, ISM control states, and the audit chain verified at export | `GET /api/v1/projects/{name}/releases/{release}/evidence`, `routes.go:2720`, `handlers/evidence_export.go` |
| N4 signing | **Tekton Chains**, deployed as an Argo CD app, signs each build's SLSA v1 provenance and image signature with a Vault Transit key (`panacea-release`); no public Sigstore, transparency log off. **Kyverno** verifies signatures at admission, in **Audit** mode | `kubernetes/argocd-apps/platform/tekton-chains.yaml`; `docs/runbooks/tekton-chains-signing.md`; `STATUS.md:135-138` |
| Drill-in UI | **the Panacea Portal is a Backstage app** behind Keycloak OIDC | `backstage/`, `deploy/portal/deploy-portal.yaml` |
| Agent access | **`panacea-mcp`**, a stdio MCP server with 25 named tools, among them `panacea_deploy`, `panacea_rollback`, `panacea_logs`, `panacea_diagnose` and `panacea_security_status`; mutating tools need `confirm: true` | `services/panacea-mcp/src/index.ts`, `README.md:144-202` |

**The one thing Panacea does not know is the brief.** Its release knows commit, digest, scans and
signatures. Nothing in it records which Define artifact the release answers. That gap is the
product-specific part, and it is small.

**Two cautions from the same source.**
- **The docs say more than the status file.** `docs-site/docs/architecture/security.md:102`
  says "All images signed; Kubernetes admission controller rejects unsigned images", but
  `STATUS.md:135-138` says Kyverno runs in Audit and `sign-release.yaml` "is written but not yet
  wired into any pipeline". **Whether Chains signs on dev today is unverified.**
- **Panacea runs Vault** (BUSL-1.1). That is Panacea's concern while it stays separate
  (`OD-13`). At the lift, `OD-32` swaps in OpenBao, which keeps a Transit engine, so the signing
  chain survives the move. This is inferred from OpenBao's feature list, not tested.

## 2. What already exists: Panacea is the deploy platform

*(pending)*

## 3. Candidates

**How to read this section.** Every product below except the signing tools would be a **second
deploy platform beside Panacea**. For our own environment that adds work instead of removing it,
so the useful question for each is narrower: does it give the cockpit something Panacea lacks, or
a deploy target a customer can host before Panacea is lifted (`OD-13`)?

**GLASSES coverage.** The catalogue (offline index and `lcfr.ai/api/mcp`, both searched
2026-09-30) lists Coolify, Dokploy, Dokku, CapRover, Portainer, Appwrite, Rainbond, Openship and
Uncloud as deploy services. It has **no entry** for Argo CD, Kargo, Backstage, Port, Sigstore,
SLSA, in-toto, Tekton or Kyverno (searches for `gitops`, `argo cd`, `attestation`, `sigstore`,
`backstage` and `developer portal` returned 0). Everything else here came from the web.

### 3.1 Coolify

- **Does:** self-hosted PaaS (Heroku/Vercel-style) on your own servers over SSH and Docker: git
  push to deploy, preview deployments per pull request, 280+ one-click services.
- **Licence:** Apache-2.0 (GLASSES, upstream metadata). Permissive.
- **Deploy:** self-host on any VM, so an Azure VM in Australia East works. **No Kubernetes**: the
  v5 plan is Coolify's own Docker-based scaling, and Docker Swarm is deprecated
  ([docs](https://coolify.io/docs/knowledge-base/docker/swarm)). So no Kata, no AKS.
- **Enterprise fit:** OIDC login arrives in **v4.4-rc.1 (2026-08-19)**, not yet stable; structured
  audit logging since v4.1.0; the only role split is Member read-only
  ([changelog](https://coolify.io/changelog)). Git sources are GitHub, GitLab, Bitbucket and Gitea;
  **Azure DevOps is not listed**, which fails `OD-4`. Preview deployments need the GitHub App.
- **Maturity:** 62,365 stars (GLASSES, 2026-09-29); v4.3.9 on 2026-08-18; one company (coollabs).
- **Gaps:** no scans, no posture gate, no evidence, no promotion model, no ADO, single-box design.
- **Effort:** 5-8 days to stand up, secure and wire, and we would still own scans and evidence.
- **Verdict: reject.** It is a good small-team PaaS and a useful reference for how simple a
  preview-deploy screen can be. It is weaker than Panacea on every regulated-enterprise axis.

### 3.2 Dokploy

- **Does:** the same class as Coolify, on Docker Swarm, with multi-server support and previews.
- **Licence: split since 2026-01-21.** Core Apache-2.0; everything under `/proprietary` is under
  a **Dokploy Source Available License**, and that is where **SSO/SAML, SCIM, custom roles, audit
  logs and white-labelling** live ([LICENSE.MD](https://github.com/Dokploy/dokploy/blob/canary/LICENSE.MD),
  [enterprise docs](https://docs.dokploy.com/docs/core/enterprise)). **RESTRICTED**: the parts an
  enterprise needs are source-available, need a licence key, and cannot be shipped into a customer
  tenancy under our permissive rule.
- **Deploy:** self-host, or Dokploy's managed Enterprise Cloud (region not published).
- **Enterprise fit:** only with the paid licence. Pricing is "contact sales".
- **Maturity:** 37,564 stars (GLASSES); commercial backing by the Dokploy team.
- **Gaps:** as Coolify, plus the licence.
- **Effort:** as Coolify, plus a commercial negotiation.
- **Verdict: reject** on licence and on fit.

### 3.3 Azure Container Apps with GitHub Actions or ADO Pipelines

- **Does:** managed containers on Azure. **Revisions** are immutable snapshots; **labels** give a
  revision its own stable URL, which is a preview per branch; **traffic splitting** is promotion
  (blue-green, canary); re-activating an old revision is rollback; 100 inactive revisions kept
  ([revisions](https://learn.microsoft.com/en-us/azure/container-apps/revisions), updated
  2026-02-26; "deployment labels" mode is **preview**).
- **Licence:** commercial Azure service. Billing is per vCPU-second, GiB-second and request, with
  a monthly free grant of 180,000 vCPU-s, 360,000 GiB-s and 2 million requests per subscription
  ([billing](https://learn.microsoft.com/en-us/azure/container-apps/billing)).
- **Deploy:** in the customer's own subscription, Australia East and Southeast available. Build
  with `azure/container-apps-deploy-action` (GitHub) or the `AzureContainerApps` task (ADO), so
  both `OD-4` hosts work.
- **Enterprise fit:** strong. Built-in Entra authentication per app, managed identity, private
  VNet environments, Azure Activity Log for audit, Defender for Containers for image scanning.
- **Maturity:** GA since 2022, Microsoft-backed.
- **Gaps:** none of Panacea's posture engine, scan set or evidence document; it would need a
  pipeline for scans (`OD-24`) and our own evidence. Running a second platform in our environment
  duplicates Panacea.
- **Effort:** 3-5 days for a second `deploy.Deployer` behind the existing conformance suite
  (revision per commit, label per branch, traffic shift for promote, activate for rollback), plus
  a scan pipeline template.
- **Verdict: integrate later, not now.** It is the best answer to **"where does a customer's app
  deploy before Panacea is lifted in"**, because the customer needs no cluster. Not needed for our
  own environment, where Panacea already does all of this.

### 3.4 Argo CD, Argo Rollouts and Kargo

- **Does:** Argo CD syncs Kubernetes to what a git repo declares; its ApplicationSet pull-request
  generator makes a preview per PR; Argo Rollouts adds canary and blue-green. **Kargo** (Akuity)
  adds the missing promotion layer: *Freight* (an image digest, a git commit and config, bundled)
  moves through *Stages* with verification at each step.
- **Licence:** all Apache-2.0. Kargo is Apache-2.0 per Akuity; the commercial **Akuity Platform**
  hosts both (add-on packs of 10 Argo CD apps plus 10 Kargo stages at $99/month; enterprise is
  "contact sales", [pricing](https://akuity.io/pricing)).
- **Deploy:** self-host on AKS, or Akuity's SaaS. Since KubeCon Europe 2026 AKS also offers Argo
  CD as a managed cluster extension (`microsoft.argocd`), **public preview**, with Entra SSO and
  workload identity to ACR and Azure DevOps
  ([announcement](https://techcommunity.microsoft.com/blog/azurearcblog/announcing-public-preview-of-argo-cd-extension-on-aks-and-azure-arc-enabled-kube/4504497)).
- **Enterprise fit:** Argo CD has OIDC (Keycloak works), RBAC, audit events. CNCF graduated.
- **Maturity:** Argo CD **3.5.3 on 2026-09-14**, quarterly minors
  ([endoflife.date](https://endoflife.date/argo-cd)). Kargo 3.7k stars, **v1.12.0-rc.2 on
  2026-09-28**, one company behind it.
- **Already in the estate:** Panacea runs Argo CD for its **own** platform apps: 18 of them under
  `kubernetes/argocd-apps/platform/`, Tekton Chains and Kyverno among them. But
  **tenant apps are deliberately not GitOps**: `ADR-021` makes provisioning "imperative via
  `panacea-api` + Tekton, **NOT** Kratix/GitOps" (`STATUS.md:6`).
- **Gaps:** adopting it for tenant deploys reverses a Panacea architecture decision and needs a
  GitOps repo per app, which is a wall of configuration the wedge forbids (`CLAUDE.md`). Kargo's
  Freight is the right *idea* for a release record, but it knows nothing of a brief.
- **Effort:** 10+ days to put tenant deploys on GitOps; 0 to keep Panacea's model.
- **Verdict: reference.** Take Kargo's Freight as the model for our release manifest (digest, commit
  and brief pin, promoted together). Use Argo CD for **our own** platform apps in phase 8, as
  Panacea does. Do not use it for tenant deploys.

### 3.5 Backstage

- **Does:** a developer-portal framework: software catalog, scaffolder (templates that create a
  repo and pipeline), TechDocs, and plugins for Kubernetes, Argo CD, GitHub Actions and Azure DevOps.
- **Licence:** Apache-2.0, CNCF incubating.
- **Deploy:** self-host (a TypeScript app you build and run); hosted versions exist (Roadie,
  Spotify Portal), with US and EU regions only as far as found.
- **Maturity:** 34.5k stars; **v1.55.3 on 2026-09-29**
  ([releases](https://github.com/backstage/backstage/releases)).
- **Already in the estate, and this is the finding:** **the Panacea Portal is a Backstage app**
  (`Projects/Panacea/backstage/`, using `plugin-catalog`, `plugin-scaffolder` and
  `plugin-techdocs`), behind Keycloak OIDC. It already has the pages phase 7 would rebuild:
  `/projects/:name/deploy`, `/environments`, `/access`, `/settings`, and components
  `DeployPage`, `DeploymentStagePage`, `BuildLogsPanel`, `SecurityScansPanel`,
  `SecurityScanReview` and `PreviewPolicyCard`
  (`backstage/packages/app/src/plugins/`).
- **Gaps:** a second UI beside the cockpit. It cannot show the brief thread unless we add a
  plugin, which would be copying work into Panacea (`OD-13`).
- **Effort:** deep links from the Deploy lens into those Portal pages: **about half a day**.
- **Verdict: integrate by deep link.** Do not build a second Backstage. Do not rebuild the
  Portal's drill-ins and governance screens in the desktop: link to them.

### 3.6 Port

- **Does:** a commercial SaaS developer portal with a flexible data model ("blueprints"),
  self-service actions and scorecards. It could model Brief → Release as entities.
- **Licence:** proprietary SaaS. **$30/seat/month (Basic, up to 50 seats) or $40 (Standard, up
  to 200)**, billed annually; Enterprise custom ([pricing](https://www.port.io/pricing)).
- **Deploy:** SaaS in **EU (AWS eu-west-1) or US (us-east-1) only**
  ([API regions](https://docs.port.io/api-reference/port-api/)). No Australian region found.
- **Enterprise fit:** SSO, SCIM on Enterprise, PrivateLink. **Fails the AU-only option**
  (`OD-24`) and "on the customer's own cloud" (`OD-5`).
- **Gaps:** a third UI (after the cockpit and Panacea's Portal) holding data that belongs in the
  Hub (`OD-3`).
- **Verdict: reject.**

### 3.7 Attestations: SLSA, in-toto, Sigstore, Tekton Chains, GitHub and Azure signing

**Already decided, not repeated here:** in-toto Statement v1 as the ledger's record format, and
SLSA Provenance v1 as the release predicate (`2026-09-28-oss-adoption/05-hub-identity.md` §2,
licence notes in `licence-hub-identity.md` D3 and D5: in-toto ships only the Apache-2.0 short
notice; SLSA's only licence is the Community Specification License 1.0). The sceptic's point
stands (`08-sceptic.md:370`): **those are formats, and nothing in the plan signs them.** This
section is about who produces, signs, stores and checks the records.

| Tool | Role | Licence | Maturity | Already in Panacea? | Verdict |
|---|---|---|---|---|---|
| **Tekton Chains** | produces signed **SLSA v1.0 build provenance** for every PipelineRun (`slsa/v2alpha3` and `v2alpha4` "correspond to the slsav1.0 spec"); stores to OCI, Tekton, Archivista and others; signs with `azurekms://` or `hashivault://`; transparency log can be off ([config](https://tekton.dev/docs/chains/config/)) | Apache-2.0 | v0.29.6 on 2026-09-24; 277 stars; Tekton is a CD Foundation project | **Yes**, as an Argo CD app, signing with Vault Transit key `panacea-release`, no public Sigstore | **use as is** |
| **cosign** | signs and verifies images and **in-toto attestations with any predicate type**; KMS keys `azurekms://`, `hashivault://`, **`openbao://`**; stores signatures as OCI 1.1 referrers, which ACR implements ([key management](https://docs.sigstore.dev/cosign/key_management/overview/)) | Apache-2.0 | **v3.1.3 on 2026-08-06**, which fixes a verification bypass (GHSA-fx35-mq7g-6g98): pin at or above it | the key and `sign-release.yaml` exist; the task is unwired | **adopt** |
| **Kyverno** | admission check: "this image carries a valid signature and an attestation of type X" (`ImageValidatingPolicy`) | Apache-2.0 | **v1.19.1 on 2026-09-10**; 8.2k stars; CNCF | **Yes, in Audit mode**, key `k8s://kyverno/panacea-cosign-pub` | **use as is; Enforce later** |
| **Notation + Ratify** (AKS Image Integrity) | Microsoft's native signing and admission path with Azure Key Vault | Apache-2.0 | AKS Image Integrity is still **preview** | Gatekeeper is deployed; Notation is not | **reject**: Panacea's runbook says "One signer only". Two signers attach conflicting predicates |
| **GitHub artifact attestations** (`actions/attest@v4`) | SLSA provenance plus **custom predicates** from a GitHub Actions run ([README](https://github.com/actions/attest)) | proprietary service; the action is open source | GA | no | **reference**: "private or internal repositories" need **GitHub Enterprise Cloud** and it is **not on GitHub Enterprise Server**. It does not help Azure DevOps (`OD-4`), so it cannot be the one path |
| **GUAC**, **Archivista with Witness** | provenance graph and attestation store | Apache-2.0 | OpenSSF / in-toto projects | no | **reference**: the Hub is the ledger (`OD-3`). A second store duplicates it |

**Three places the brief pin can live, cheapest first.**

1. **In the Hub's release record, as a signed in-toto Statement.** Subject: the image digest
   Panacea reports for the release. Predicate (ours): brief key, revision and sha256, the release
   id, the `Panacea-Session` list, and the **sha256 of Panacea's evidence document** for that
   release. The Hub signs it server-side with cosign and a Key Vault key (`azurekms://`), and later
   an OpenBao Transit key (`OD-32`, `openbao://`). **Needs no Panacea change.**
2. **On the image in ACR, as an OCI referrer.** The same Statement pushed with `cosign attest`. An
   assessor with `cosign verify-attestation` sees the brief without our Hub. **Needs push rights to
   Panacea's registry**, which is a Panacea change.
3. **Inside Chains' signed SLSA provenance.** Chains records the PipelineRun's parameters, so if
   `POST P/deploy` accepted one more field and passed it through, the brief pin would sit inside
   provenance **signed by the platform key**, and Kyverno could enforce **"no brief, no prod"**. Today
   `TriggerDeployOptions` has no such field (`models/models.go:2342-2372`: branch, commit, noCache
   and ten scan modes). That is exactly the plan's **metadata ask** (R7, Q-P7-2, task 20).
   That Chains records parameters is from its format documentation and **was not checked against
   Panacea's pipeline**.

**Known risk:** Kyverno issue
[#17363](https://github.com/kyverno/kyverno/issues/17363) reports that KMS-key verification fails in
v1.19.0 for signatures stored **only as v0.3 Sigstore bundle referrers**, which is how cosign v3
stores them. Test with the exact versions before any Enforce flip.

### 3.8 Others checked

| Candidate | What it is | Licence and price basis | Verdict and the one reason |
|---|---|---|---|
| **Octopus Deploy** | release management: promotion lifecycles, deployment freezes, ITSM change approvals (ServiceNow, Jira Service Management), SIEM audit streaming, Entra SSO, an Argo CD integration. An Australian company | proprietary; **per project**: Cloud Professional $4,330/yr up to 100 projects, Cloud Enterprise $24,600/yr, self-hosted Server Enterprise $15,600/yr ([pricing](https://octopus.com/pricing/overview)) | **reference, buy only on demand.** The one thing it has that Panacea lacks is a ServiceNow change ticket on promotion. If a customer mandates that, buy Server Enterprise and self-host it in their tenancy. Octopus Cloud's hosting region was not found |
| **Harness** (CD, GitOps, Supply Chain Security) | a whole delivery suite with SLSA provenance | proprietary; modular, per service or developer; one third-party estimate is $450K-$650K/yr for 500 services | **reject.** It duplicates Panacea, and its SLSA Level 3 is "not possible in SMP", its self-managed edition ([Harness docs](https://developer.harness.io/docs/software-supply-chain-assurance/ssca-supported/)) |
| **Northflank**, **Qovery** | bring-your-own-cloud PaaS that runs AKS in your subscription: preview environments per PR, release flows | proprietary; Northflank pay-as-you-go $0.01667/vCPU-hour; Qovery per plan | **reject.** A second platform whose control plane sits outside the customer's tenancy |
| **Bunnyshell**, **Release**, **Okteto**, **Signadot** | preview environments as a service | proprietary SaaS | **reject.** Panacea already gives every branch an environment. Not examined in depth |
| **Dokku**, **CapRover**, **Portainer**, **Rainbond**, **Openship**, **Uncloud**, **Appwrite** | the other GLASSES "deploy-service" entries: Docker PaaS or container managers | various (GLASSES shows MIT, Zlib, Apache-2.0, BSD-3-Clause; two unknown) | **reject** for the reasons in 3.1. Not examined further |
| **GitHub Environments and Deployments API; ADO Environments** | a deployment record in the git host, with approvals | part of GitHub and Azure DevOps | **later, optional glue (about 1 day).** Write each Ship as a deployment in the repo's host, so a developer sees "deployed to preview, brief B" where they already work |
| **`panacea-mcp` inside the sandbox** | lets an agent read its own preview's logs, pods and diagnosis, and fix a failing deploy itself | Panacea's own | **later.** Its token is a user's `pnc_` key with that user's role, and `panacea_api_request` can write. Agents are untrusted, so this waits for a read-only, per-session token, as phase 6 does for inference |

## 4. Side by side

*(pending)*

## 5. Recommendation

*(pending)*

## 6. Calls only the owner can make

*(pending)*

## 7. What this study did not verify

*(pending)*
