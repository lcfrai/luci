# 06 — Deploy and release provenance: what to adopt instead of building phase 7

**Status:** in progress, 2026-09-30, **second run** (relaunched to take the owner's second
direction; the first run stopped at 09:47 with sections 1-4 written). Research only: nothing was
installed or run, and nothing was written in `Projects/Panacea`.
**Capability:** shipping the result — a preview environment per branch, promotion, rollback, and a
release that carries its provenance (the brief it answers, the evidence).
**Replaces (candidate):** phase 7, the Deploy connector, 20 tasks
(`.claude/worktrees/phase-7/docs/superpowers/plans/2026-09-23-phase-7-deploy.md` §8).
**Second direction (owner, 2026-09-30):** Panacea is the strong default and the best fully-fledged
solution wins; Coolify plus Supabase enhanced with Panacea snippets is allowed; Panacea snippets may
be copied with attribution (OD-13 relaxed); Orca is the cockpit. What that changed is in section 8.
**Companion:** `00-panacea.md` §3-5 holds the full route and tool inventory with `file:line`; this
file uses it and adds only what deploy needs.

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
`P/promote`** (plan line 480), so N2 is not in it today. **Built so far** *(second run, from the
branch's log)*: `feat/phase-7-deploy` holds tasks 01-06 (domain, transport, reads, acts, connection,
binding: commits `70c66ecc`, `5940041c`, `fd953dd6`, `a1ecd4f8` and their fix rounds) and part of
07, the watcher (`16f189db`, `db9cc518`, `0477c5a4`). That is about 14 of the 51 hours, not the
"4 of 20" in the brief for this study. Nothing of it is merged.

**What that means for this study.** Phase 7 is small because the deploy product already exists:
it is Panacea. The question is not "which platform replaces a 20-task build" but **"what else
would shorten the remaining 16 tasks, close N2 and N4, and give us a deploy target a customer can
host"**.

## 2. What already exists: Panacea is the deploy platform

Read from Panacea's source at `efc637879` (2026-09-30), read-only. Nothing was run.

| Need | What Panacea already has | Where |
|---|---|---|
| N1 preview per branch | per-branch environments, a preview policy, per-branch auto-deploy, access posture (SSO, IP list, share links), URL prefix | `routes/routes.go:1162-1164`, `:1308-1309`, `:1424-1442`, `:1313-1314` |
| N1 data per preview *(second run)* | **copy-on-write database branching**: a preview branch gets its own CloudNativePG cluster recovered from production's backup, opt-in via `panacea.io/branch-database=true`, "honest degrade" to the shared DB; **sanitised data previews** (D10): a redaction pass before the app gets a connection string | `clients/db_branch_ops.go:17-41`; `clients/branch_data_ops.go:15-46`; `STATUS.md:91-95` |
| N1 ship | `POST P/deploy` starts a Tekton pipeline with scans; cancel | `routes.go:1286`, `:1291` |
| N2 promote | `POST /api/v1/projects/{name}/promote` and `POST …/envs/{env}/stage/promote` | `routes/routes.go:1027`, `:2818` |
| N2 gate | a posture-policy engine: platform baseline plus team/project tightening, per-check `block` or `ack`, fail-closed, gating **prod-deploy** | `STATUS.md:125-135`; `handlers/security_policy.go` |
| N2 ladder *(second run)* | a **deployment-stage ladder**, innovation < secure-test < production, each a bundle of existing controls: scans per action, SSO, data-class ceiling (`none` < `synthetic` < `protected`), risk state, backup, and the approver (none, owner, SuperAdmin) | `handlers/deployment_stages.go:39-97`, bundles `:176-259` |
| N3 rollback | `POST P/deployments/{id}/rollback`, with warmth and cold-start seconds | `routes.go:1026`; `handlers/projects.go:1547` |
| N4 evidence | **a per-release evidence document**: release identity (digest, commit, env, PipelineRun), the recorded scan verdicts for *that* release, signature and SBOM references "honestly absent when they did not" exist, ISM control states, and the audit chain verified at export | `GET /api/v1/projects/{name}/releases/{release}/evidence`, `routes.go:2720`, `handlers/evidence_export.go` |
| N4 signing | **Tekton Chains**, deployed as an Argo CD app, signs each build's SLSA v1 provenance and image signature with a Vault Transit key (`panacea-release`); no public Sigstore, transparency log off. **Kyverno** verifies signatures at admission, in **Audit** mode | `kubernetes/argocd-apps/platform/tekton-chains.yaml`; `docs/runbooks/tekton-chains-signing.md`; `STATUS.md:135-138` |
| Drill-in UI | **the Panacea Portal is a Backstage app** behind Keycloak OIDC | `backstage/`, `deploy/portal/deploy-portal.yaml` |
| Agent access | **`panacea-mcp`**, a stdio MCP server with **31 tools** (the first run said 25; the source has 31 `name: "panacea_…"` entries), among them `panacea_deploy`, `panacea_rollback`, `panacea_logs`, `panacea_diagnose`, `panacea_security_status` and the `panacea_api_request` escape hatch; mutating tools need `confirm: true` | `services/panacea-mcp/src/index.ts:249-578`; `00-panacea.md` §4.1 |

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

## 2a. Each need mapped onto Panacea's routes and MCP tools

`P` = `/api/v1/projects/{name}`; lines are `services/panacea-api/routes/routes.go` unless named;
tools are `services/panacea-mcp/src/index.ts`. Re-read in Panacea's tree on 2026-09-30 (grep only).
**Who** says which side may act: the **agent** (untrusted, through `panacea-mcp` and a scoped token)
or a **human** (a held act in the cockpit, or Panacea's Portal).

| Need | `panacea-api` route | `panacea-mcp` tool | Who | Gap for us |
|---|---|---|---|---|
| N1 ship a branch | `POST P/deploy` `:1286` (developer + section-writable); `POST P/code` `:1033` pushes files and deploys; per-branch auto-deploy `:1308-1309` | `panacea_deploy` `:404` | agent for a preview; human for the production branch | Panacea builds only **pushed** commits (plan §8, `clients/pipeline.go:174`), so phase 3's push comes first; the request carries no brief (`models/models.go:2342-2372`) |
| N1 watch and fix | run `:1290`, logs `:2288`, log SSE `:2283`, readiness SSE `:2286`, AI failure analysis `:2291`, pods and events `:1148-1155` | `panacea_logs`, `panacea_diagnose`, `panacea_events`, `panacea_pods_list`, `panacea_deploys_latest` | agent | none: this is the loop that lets an agent fix its own failed deploy |
| N1 preview policy and access | previews `:1162`, preview policy `:1163-1164`, branch SSO and IP list `:1424-1426`, access `:1430-1431`, share links `:1440-1442`, reviewer comments `:2931-2939` | only through `panacea_api_request` `:578` | human (owner-level settings) | no named tool; these are settings screens the Portal already has |
| N1 data per preview | none: opt-in by the project label `panacea.io/branch-database=true` (`handlers/preview_data.go:31-35`) | none | platform | **no route writes the label** after creation; `CreateProjectRequest.Labels` (`models/models.go:650`) passes labels through at create (`handlers/projects.go:360-374`), so it can plausibly be set then. Not tested |
| N2 promote | env to env `POST P/promote` `:1027` (`handlers/projects.go:1643-1690`); stage ladder `GET/PUT P/envs/{env}/stage` `:2814-2816`, `POST …/stage/promote` `:2818`, `POST …/stage/approve` `:2860` (owner route; the production rung wants SuperAdmin, `deployment_stages.go:259`); project floor `PUT P/stage-floor` `:2862` (owner) | none named; `panacea_api_request` reaches them | **human only** | an agent's developer-capped token can call `P/promote`; see the allowlist in §6 |
| N2 gate by data class (`OD-24`) | a promote **to production** runs the budget gate (`projects.go:1674`), the risk gate, then posture **and** stage (`projects.go:2894-2927`); the stage bundle holds the data-class ceiling | `panacea_security_status` | platform | **no route writes `panacea.io/data-classification`** (03-define G7 hands this step to us); production is **unreachable by construction until DLP label enforcement is live** for the project (`deployment_stages.go:46-59`, `STATUS.md:85-86`) |
| N3 rollback, redeploy | rollback `:1026` (`projects.go:1547`), redeploy `:1028`, cancel `:1291` | `panacea_rollback` `:439`, `panacea_redeploy` `:425` | agent on a preview; human on production | none |
| N3 history | `GET P/deployments` `:1024`, `GET P/deploys/latest` `:1025` (`version`, `commit`, `branch`, `environment`, `failedStep`) | `panacea_deploys_list` | both | none |
| N4 evidence | `GET P/releases/{release}/evidence` `:2720` (`handlers/evidence_export.go`) | only through `panacea_api_request` | both | **knows no brief**; signatures "honestly absent" while `sign-release.yaml` is unwired (`STATUS.md:135-138`) |
| N4 agent instructions | `GET P/agent-context` `:1086`, `GET P/skill-md` `:1072-1073`, `GET /api/v1/docs/agents` `:642` | — | agent | none: an agent learns how to deploy here from Panacea itself |
| Identity for all of it | a `pnc_` token (`middleware/api_key.go:19`); minting needs the user's SSO session (`handlers/api_keys.go:112-116`) | `PANACEA_TOKEN` | — | our realm token is refused on REST (`middleware/auth.go:245-251`); 00-panacea §5.3 ranks the ways through |

**Five gaps, and whose they are.**

| # | Gap | Size | Who closes it |
|---|---|---|---|
| P-1 | The deploy request carries no brief pin | 0.5-1 day in Panacea: one optional field, passed to the PipelineRun as a parameter so Chains signs it | Panacea stream (the plan's metadata ask, task 20) |
| P-2 | No route writes `panacea.io/data-classification` | about 0.5 day in Panacea: an owner-gated, audited `PUT` | Panacea stream; until then we set the **stage floor**, which is the part `OD-24` needs (scans) |
| P-3 | Our realm token is not trusted, so a sandbox cannot get its own short-lived token | 1-2 days in Panacea (00-panacea §5.3, option 1) | Panacea stream; until then one pasted `pnc_` per user behind our proxy |
| P-4 | Release signing is written but not wired; Kyverno verifies in Audit | Panacea's own backlog (`STATUS.md:135-138`) | Panacea stream; our Hub-side signature (§6) does not wait for it |
| P-5 | An environment on the **production rung** cannot be reached until DLP enforces for the project. An environment with no rung set sits on **innovation** (`deployment_stages.go:500-518`), where a production deploy needs no scan at all | Panacea's DLP track 6p (`STATUS.md:87-89`) | Panacea stream; an owner call on what "production" means at first ship (§7) |

## 3. Candidates

**How to read this section.** Every product below except the signing tools would be a **second
deploy platform beside Panacea**. For our own environment that adds work instead of removing it,
so the useful question for each is narrower: does it give the cockpit something Panacea lacks, or
a deploy target a customer can host before Panacea is lifted (`OD-13`)?

*Revised for the second direction.* The owner allows **Coolify plus Supabase, enhanced with
Panacea snippets**, as a whole alternative, judged on merit. 3.1 and 3.2 keep the product facts;
**3.9 judges that stack as a whole deploy step**, 3.10 looks at Supabase for preview data, and
3.11 says how deploy shows up in Orca. Verdicts changed in place are marked *(revised)*.

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
- **Maturity:** 62,408 stars; latest stable **v4.3.23 on 2026-09-18**, and v4.4 (OIDC, a team
  audit log, third-party secret managers) is **still a release candidate**, `v4.4-rc.1` of
  2026-08-19 (GitHub API, 2026-09-30); one company (coollabs).
- **Azure DevOps** *(second run)*: not a native source. The "other git providers" path is a deploy
  key plus a webhook ([docs](https://coolify.io/docs/applications/ci-cd/other-providers)), which
  auto-deploys on push; **preview deployments per pull request need the GitHub App**.
- **Gaps:** no scans, no posture gate, no evidence, no promotion model, no ADO previews,
  single-box design.
- **Effort:** 5-8 days to stand up, secure and wire, and we would still own scans and evidence.
- **Verdict: reject** *(confirmed on merit under the second direction, 3.9)*. It is a good
  small-team PaaS and a useful reference for how simple a preview-deploy screen can be. It is
  weaker than Panacea on every regulated-enterprise axis, and Panacea snippets do not close that
  (3.9).

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
- **Maturity:** 37,578 stars; GitHub reports the licence as `NOASSERTION`, which fits the split
  (API, 2026-09-30); commercial backing by the Dokploy team.
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
- **Verdict: reference** *(revised; the first run said "integrate later")*. The first run made it
  the answer to "where does a customer's app deploy before Panacea is lifted in", because the
  customer needs no cluster. Under the second direction Panacea is the deploy step, `OD-5` puts
  customer hosting later, and the customer-hosted tier `CLAUDE.md` names is an **Azure Kubernetes
  App**, which is the lifted Panacea. So ACA is not in the plan. Revisit it only if a customer
  refuses a cluster: it would then be a second `deploy.Deployer` behind the existing conformance
  suite, 3-5 days, with no scan gate and no evidence of its own.

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

### 3.9 Coolify plus Supabase, enhanced with Panacea snippets, as the whole deploy step *(second run)*

The owner's allowed alternative, judged as a complete answer rather than product by product.
**The stack:** Coolify (Apache-2.0) on Azure VMs in Australia East deploys containers to Docker
hosts; self-hosted Supabase (Apache-2.0) gives each app its database, auth and storage; attributed
Panacea snippets fill what Coolify lacks. Every part is permissive, so licence is not the issue.

| Need | Coolify and Supabase give | Panacea snippet to port | Days (estimate) |
|---|---|---|---|
| Entra sign-in | Coolify OIDC is **release-candidate only** (3.1) | oauth2-proxy in front, on Panacea's Keycloak chain | 1 |
| Scans before promote | none | Panacea's scan tasks (`pipelines/tasks/*-scan.yaml`, 18 checks) rewritten as ADO and GitHub pipeline steps | 3-5 |
| Promotion gated by data class | none | `handlers/security_policy.go` and `deployment_stages.go` lifted into a gate service in front of Coolify's API; both read Panacea's Kubernetes ConfigMaps, scan-result source and risk resolver, so each input is rewired | 4-6 |
| Evidence per release | none | `handlers/evidence_export.go`, which reads Panacea's scan records and audit chain, rewired to Coolify's data | 2-3 |
| SSO- or IP-gated previews | preview URLs, public by default | one oauth2-proxy per preview, as Panacea's branch security does | 1-2 |
| Previews for Azure DevOps (`OD-4`) | GitHub App only | pipeline-driven previews through Coolify's API | 2 |
| A database per preview | Supabase Branching is **hosted only** (3.10) | Panacea's branching is CloudNativePG on Kubernetes, so it does not port to Docker: `pg_dump` and restore per preview instead, and D10's redaction pass on top | 2-4 |
| Signing | none | cosign in the pipeline, as in 3.7 | 1 |
| Isolation of agent-written apps | containers on shared Docker hosts | **none portable**: Panacea's tenant namespaces, NetworkPolicies, quotas, Kyverno and Falco are Kubernetes controls | not closable |
| **Total** | | | **16-24 days**, and weaker isolation at the end |

**Where the stack wins.** A simpler first screen, 280+ one-click services, one VM to run, no
cluster. For a small team shipping trusted code that is a good trade.

**Where it loses, for our customer.** A regulated enterprise needs the half of deploy that Coolify
does not have: scans gated by data class, evidence, SSO-gated previews, Azure DevOps previews and
tenant isolation for code an untrusted agent wrote. Panacea has all of it running (§2, §2a).
Porting it onto Coolify is rebuilding Panacea's deploy side on a platform that was not designed
for it, at 3-4 times the days of using Panacea.

**Trade-off in one line:** Coolify is quicker to click and cheaper to host on one VM; Panacea
already has the regulated half, which Coolify would need 16-24 days of ported Panacea code to
approach, on a weaker isolation model.

**Verdict: reject as the deploy step.** Keep Coolify as the reference for how short a
preview-deploy screen can be (the Orca panel, 3.11, should feel that short).

### 3.10 Supabase, for preview data and the shipped app's backend *(second run)*

- **Does:** a Postgres platform (auth, PostgREST, realtime, storage, edge functions). **Branching**
  gives a pull request its own preview database.
- **Licence:** Apache-2.0 (110,905 stars, pushed 2026-09-30, GitHub API).
- **Deploy:** Supabase Cloud on AWS, Sydney region available (03-define §3); self-host with
  Docker Compose. **Branching is "N/A" for self-hosted** and Git-based branching needs the Pro plan
  and GitHub ([branching docs](https://supabase.com/docs/guides/deployment/branching)).
- **Enterprise fit for this need:** preview databases only exist in the hosted, AWS-run service,
  which misses `OD-18` (Azure) and puts customer data in a third-party SaaS.
- **Gaps:** Panacea already branches a preview's database copy-on-write from production's backup
  and redacts it before the app sees it (§2, `db_branch_ops.go`, `branch_data_ops.go`). Supabase
  adds nothing to deploy that Panacea lacks.
- **Verdict: reject for deploy.** Whether a shipped app is *built on* Supabase is a Develop-phase
  template choice, and a Supabase app still deploys through Panacea like any other container.
  03-define and 05-work-team hold Supabase as a reference for the Hub store.

### 3.11 Orca, as the surface deploy shows up in *(second run; the pick is not re-argued)*

Orca (`stablyai/orca`, **MIT**, 81,623 stars, pushed 2026-09-30) is the owner's cockpit. Its facts
are 02-fleet-cockpit's (§R2, read from source at `59b746ff`); what matters here is where deploy
plugs in. Phase 7 planned a Tauri Deploy lens (tasks 13-18, 17 plan-hours). In Orca it becomes
three things, and only the third is built:

1. **The agent deploys, with no UI.** In an Orca worktree session, an unmodified Claude Code
   calls `panacea_deploy` for its branch's preview, reads `panacea_logs` and `panacea_diagnose`,
   and fixes a failed build itself. `panacea-mcp` ships in the sandbox image (01-sandboxes, day 1
   row) with a session token that reaches only an allowlist (§6, G4).
2. **The preview opens in Orca's per-worktree browser** (04-design G3), with a silent Keycloak
   sign-in when the preview's access mode is SSO.
3. **One Deploy panel** (§6, G1): the release card (brief pin, sessions, evidence, signature),
   the held human acts (ship the production branch, request promotion, roll back) through our
   control plane, and deep links into Panacea's Portal for logs, scans, governance and approval.

**Uncertain, and not ours to settle:** 02-fleet-cockpit found an experimental Plugin API v1 whose
panels are sandboxed frames, with `net:fetch` "in later phases"; 03-define (§4.11, G5) found no
documented plugin API and plans changes in an Orca fork. The Deploy panel is the same 1.5 days
either way: a plugin panel if its sandbox can reach our control plane, otherwise a widget in the
fork 03-define already needs. The cockpit cluster owns the choice.

**Verdict: integrate.** Also relevant: Orca creates Azure DevOps pull requests (02 §R2), so a
lane lands on the host before Panacea builds it, which is the order Panacea needs (it builds
pushed commits only).

## 4. Side by side

N1 preview per branch · N2 promote · N3 rollback · N4 release carries brief and evidence.
"Days" is engineer-days to reach N1-N4 **from today**, with Panacea's existing API counted as done.

| Candidate | Licence | Runs where | Entra SSO | AU-only possible | N1 | N2 | N3 | N4 | Days | Verdict |
|---|---|---|---|---|---|---|---|---|---|---|
| **Panacea** (API, `panacea-mcp`, Portal, Chains, Kyverno) | ours | its own AKS | yes, Keycloak | yes | yes, with a database per preview | yes, `P/promote` and the stage ladder | yes | evidence yes, **brief with our glue** | **6** (§5) *(revised from 5)* | **adopt** |
| Coolify | Apache-2.0 | any VM, Docker | OIDC in an RC | yes | GitHub only | no | yes | no | 5-8 and still no N4 | reject |
| Dokploy | Apache-2.0 core, **source-available** enterprise | VM, Swarm | paid only | yes | yes | no | yes | no | 5-8 plus a contract | reject |
| **Coolify + Supabase + Panacea snippets** *(second run, 3.9)* | all permissive | Azure VMs, Docker | via oauth2-proxy | yes | yes, after 2 days of ADO work | after porting the posture engine | yes | after porting evidence | **16-24**, isolation still weaker | reject on merit |
| Supabase for preview data *(3.10)* | Apache-2.0 | branching hosted only, AWS | plan-dependent | Sydney, not Azure | databases only | n/a | n/a | n/a | n/a | reject for deploy |
| Orca as the surface *(3.11)* | MIT | the cockpit | via our sign-in | yes | the agent ships, the browser shows | a held act | a held act | shows the card | 1.5 (inside the 6) | **integrate** |
| Azure Container Apps + Actions or ADO | Azure service | customer subscription | yes, built in | yes | yes, labels | yes, traffic split | yes | no | 3-5 for a second `Deployer` | reference *(revised)*: only if a customer refuses a cluster |
| Argo CD + Kargo | Apache-2.0 | AKS | yes | yes | yes | yes | yes | no brief | 10+ | reference |
| Backstage | Apache-2.0 | already Panacea's Portal | yes | yes | shown | shown, with the approve step | shown | no | 0.5 for links (inside G1) | **integrate by link** |
| Port | proprietary | SaaS, EU or US | yes | **no** | no | no | no | no | n/a | reject |
| cosign + in-toto + Kyverno | Apache-2.0 (in-toto short notice) | wherever we sign | n/a | yes | n/a | n/a | n/a | **yes** | ~1 (inside the 6) | **adopt** |
| Tekton Chains | Apache-2.0 | already in Panacea | n/a | yes | n/a | n/a | n/a | build yes, brief only with the ask | 0 | use as is |
| GitHub attestations | GitHub service | GitHub Enterprise Cloud | yes | depends on the GitHub region | n/a | n/a | n/a | yes, GitHub only | 1 | reference |
| Octopus Deploy | proprietary, per project | cloud or self-hosted | yes | self-hosted yes | via targets | yes, lifecycles | yes | work items, no brief | 10+ | reference, buy on demand |
| Harness | proprietary | SaaS or self-managed | yes | self-managed | yes | yes | yes | SLSA, SaaS only | 15+ | reject |

## 5. Recommendation

**Adopt Panacea as the deploy step, through `panacea-mcp` for agents and its API for the cockpit.
Finish phase 7 as a thin connector cut down for Orca, and sign the brief-to-release link in the
Hub with cosign and an Azure Key Vault key. About 6 engineer-days** (3.5 with two people: the
control plane and the Orca panel run in parallel).

**Why this one, on merit.** Panacea is the only candidate that already meets N1-N3 in full and
most of N4, in our own Azure estate, behind Entra through Keycloak, with Azure DevOps and GitHub:
per-branch previews with SSO, IP and share-link access; a database branched per preview and
redacted before the app sees it; a stage ladder that turns `OD-24`'s data class into the scans a
promotion needs; rollback with warmth; and an evidence export per release (§2, §2a). The best
alternative the owner allowed, Coolify plus Supabase with Panacea snippets, needs 16-24 days of
ported Panacea code to approach that and still isolates agent-written apps less well (3.9).
Nothing else examined adds a need Panacea misses, except a ServiceNow change ticket (Octopus,
3.8), which waits for a customer who mandates it.

**What it replaces in the plan.** Phase 7, the Deploy connector (20 tasks, about 51 plan-hours):

| Phase 7 tasks | What happens | Plan-hours |
|---|---|---|
| 01-06 | **built** on `feat/phase-7-deploy`, kept | 14 |
| 07 watcher (part built), 08 view, 09 ship, 10 rollback, rebuild and cancel, 19 sessions in a release, 20 seams and live recipe | **kept**, finished as planned | 13.5 left |
| 11 governance routes, 12 drill-in routes | **replaced** by Portal deep links and `panacea-mcp`'s read tools | 4.5 |
| 13-18, the Tauri Deploy lens (data layer, frame, environment card, Ship bar, release drawer, governance panel) | **replaced** by one Orca Deploy panel (G1) and the Portal's own pages | 17 |

So **8 of the 20 tasks (21.5 plan-hours) leave the plan**, and four pieces of glue arrive that the
plan never had (§6): promotion through the stage ladder (the plan left `P/promote` out, so N2 was
unmet), a signature on the brief-to-release link (nothing in the plan signed), the allowlist that
keeps an agent's token off production, and the data class handed to Panacea (03-define G7).

**When it becomes usable.** Day 1: an agent in an Orca session deploys its branch to a preview,
reads the logs and fixes a failed build, once the sandbox image carries `panacea-mcp` (01-sandboxes)
and G4 is in. Day 3.5: a human ships the production branch from the Deploy panel, and the release
card shows the brief it answers. Day 6: promotion through the ladder and the signed provenance
record, exportable to an assessor.

**Risks, and what holds if they land.**
- **Five Panacea-side gaps belong to the Panacea stream** (§2a, P-1 to P-5). None blocks the
  first ship: the Hub-side signature does not need P-1 or P-4, the stage floor covers P-2's scan
  part, the proxy covers P-3. What waits is admission-time "no brief, no prod" (P-1 plus P-4) and
  production for protected data (P-5).
- **The release record lives in the Hub** (`OD-3`), 6 of 50 tasks built. Until the Hub's release
  routes run (H5), tasks 09 and 19 run against its fake, and the control plane keeps the signed
  records in its own state file, migrated on arrival.
- **Our customers' apps share Panacea's cluster and its incidents**, and its single Keycloak
  replica logs everyone out on a pod move (00-panacea §5.1). That is the price of "running today".
- **Sandboxes reach Panacea's API only once our egress IP is on Panacea's edge allowlist**
  (01-sandboxes, `deploy/main.tf:580-602`), a request to the Panacea side.
- **Orca's panel API is experimental**, and two clusters disagree on whether it exists (3.11);
  the fork widget is the fallback at the same cost.

## 6. The glue we still build

| # | Glue | Where | Replaces or adds | Days | Needs first |
|---|---|---|---|---|---|
| G1 | **Orca Deploy panel**: the release card, the held acts (ship the production branch, request promotion, roll back), the preview link into Orca's per-worktree browser, deep links into the Portal's `/projects/:name/deploy`, `/environments`, `/access` and stage pages | Orca plugin panel, or our Orca fork (3.11) | replaces tasks 13-18 (17 h) and the first run's 0.5-day links | 1.5 | task 08's view; the cockpit cluster's panel-or-fork call |
| G2 | **Signed release provenance**: one in-toto Statement v1 per release, DSSE-signed in the Hub with an Azure Key Vault key (`azurekms://`), `OD-32`'s OpenBao Transit key (`openbao://`) at the lift | Hub, called by task 09 on confirm | adds: nothing in the plan signed | 1 | Hub release record (fake until H5) |
| G3 | **Promotion and the data class**: at bind, `PUT P/stage-floor` from the project's class (`none` to innovation, `synthetic` to secure-test, `protected` to production); a promotion request is `POST P/envs/{env}/stage/promote`, the card shows `GET P/envs/{env}/stage`'s per-rung checklist, approval stays in the Portal | control plane, next to task 10 | adds N2, which the plan left out; closes 03-define G7's hand-off | 0.75 | task 04's transport |
| G4 | **The agent allowlist**: the sandbox's `panacea-mcp` talks to a control-plane proxy that holds the real token and forwards only the session's own project, and only the routes in the list below | control plane | adds the control that keeps an untrusted agent off production | 1 | 01-sandboxes' image and `/bind` |
| — | Phase 7 tasks 07 (rest), 08, 09, 10, 19, 20, as planned | control plane | kept | 1.7 (13.5 h) | as the plan says |
| | **Total** | | | **about 6** | |

**G2's statement, field by field.** Subject: the image digest in Panacea's evidence document for
the release. Predicate type: a URI we own. Predicate: the brief's key, revision and sha256 (the
pin Define published); the release id; the Panacea project, deployment id, environment and
commit; the sessions from the `Panacea-Session` trailers (task 19); the **sha256 of Panacea's
evidence document** (`GET P/releases/{release}/evidence`), so the scans, SBOM and signature state
Panacea recorded are bound in without being copied; the rung and the approver at promotion. An
assessor checks it offline with `cosign verify-blob-attestation` and the public key, with no
access to our Hub ([cosign attestation docs](https://docs.sigstore.dev/cosign/verifying/attestation/)).
When Panacea takes P-1, the same pin also travels inside Chains' platform-signed SLSA provenance,
and Kyverno can enforce "no brief, no prod" at admission; G2 stays as the record that also names
the sessions and the evidence digest.

**G4's allowlist, v1.** The proxy forwards, for the session's project only: every read the agent's
loop needs (deployments, runs, logs and their SSE, pods, events, diagnose, security status,
`agent-context`, `skill-md`); `POST P/deploy` for a **non-production** branch; rollback and cancel
on a **non-production** environment. It refuses everything else, among it `P/promote`, the stage
and stage-floor routes, access and preview policy, custom domains, webhooks, env writes
(`panacea_env_set`), token minting and every admin route. The reason is in §2a: a developer-capped
token *can* call `P/promote`, and on an environment with no rung set a production deploy asks for
no scan. Until P-3 the real token is the user's pasted `pnc_`, which never enters the sandbox;
after P-3 it is a per-session `scopeProject` + `scopeRole=deploy` token that expires in hours, and
the allowlist stays, because that role can still promote.

**Asks of the Panacea stream (not our days):** P-1 the brief-pin deploy parameter (0.5-1 day),
P-2 a route that writes the data class (about 0.5), P-3 trust for our realm token (1-2), and our
egress IP on Panacea's edge allowlist. P-4 and P-5 are Panacea's own backlog.

## 7. Calls only the owner can make

Business calls only: money, or what a customer sees. Everything technical above is decided.

1. **What "production" means for a customer at first ship (customer-visible).** Panacea's ladder
   puts protected data on the production rung, and that rung opens only when Panacea's DLP
   enforcement is live for the project (P-5). **Recommend:** first customer releases go to the
   **secure-test** rung (SSO enforced, enforced scans, synthetic data only) and are called a
   *pilot*; production for protected data follows Panacea's DLP track. The alternative, leaving
   production environments with no rung, is live today but asks for no scan on a production
   deploy, which a regulated customer should not be offered.
2. **Whether users see Panacea's Portal (customer-visible).** v1 deep-links from the Orca panel into
   Panacea's Backstage pages for logs, scans, governance and approval, so users meet a second,
   Panacea-branded UI. **Recommend yes for v1.** The alternative is building those screens in
   Orca: tasks 12, 17 and 18 come back, about 2-3 more days.
3. **Who pays to run shipped apps (money).** Every release is a Panacea project, billed through
   Panacea's job code and team budget (`models/models.go:652-660`); a promotion pauses when the team
   is over budget (`handlers/projects.go:1672-1676`). A database per preview needs point-in-time
   recovery, which the shared database tier does not have (`clients/db_branch_ops.go:36-41`), so
   branching previews costs the dedicated tier. **Decide:** the default job code and budget for a
   PanaceaApp project, and whether previews of protected-data projects get the dedicated tier.
4. **Later, only on demand (money):** if a customer mandates a ServiceNow change ticket per
   promotion, buy Octopus Deploy Server Enterprise ($15,600 a year as listed, self-hosted, 3.8)
   rather than build it.

## 8. Revised for the owner's second direction

The first run (stopped 09:47) already made Panacea the deploy step. The second direction did not
reverse that; it asked that the answer be **earned against the allowed alternative** and fitted
to **Orca**. What changed, and why:

| What | First run | Now | Why |
|---|---|---|---|
| The comparison | Coolify rejected on its own | Coolify **plus Supabase plus Panacea snippets** costed as a whole deploy step: 16-24 days, weaker isolation (3.9) | the owner allowed that stack; judged on merit it still loses |
| Supabase | not examined | reject for deploy (3.10) | branching is hosted-only on AWS; Panacea already branches and redacts preview data |
| The cockpit surface | a deep link from the Tauri Deploy lens | one Orca panel (G1); tasks 13-18 leave | Orca is the owner's cockpit |
| Azure Container Apps | integrate later, for customer tenancies | reference, only if a customer refuses a cluster | Panacea is the strong default, and the customer-hosted tier is the lifted Panacea on AKS |
| Promotion (N2) | noted as missing from the plan | built as G3 on Panacea's **stage ladder**, found this run | the ladder is `OD-24`'s "class decides the scans", already running |
| Data per preview | not examined | Panacea's copy-on-write database branching, found this run | the Supabase question made us look |
| Agent tokens | "later" (3.8) | G4, the allowlist, in v1 | an agent in every Orca session can reach `P/promote` without it |
| Days | 5 | about 6 | G3 and G4 are new; the Orca panel replaces 17 plan-hours of Tauri lens |
| Facts | 25 MCP tools; 4 of 20 tasks built | 31 tools; tasks 01-06 and part of 07 built | re-read from source and the branch log |

**What the relaxed `OD-13` changes here: nothing we need to copy.** Deploy calls Panacea rather
than copying it. The snippets priced in 3.9 are what Coolify would need, which is the argument
against it. If the lift happens, the connector's `deploy.Deployer` seam makes it a swap.

## 9. What this study did not verify

*(pending)*

## 10. Sources

*(pending)*
