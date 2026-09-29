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

### 3.1 Coolify
*(pending)*

### 3.2 Dokploy
*(pending)*

### 3.3 Azure Container Apps with GitHub Actions or ADO Pipelines
*(pending)*

### 3.4 Argo CD, Argo Rollouts and Kargo
*(pending)*

### 3.5 Backstage
*(pending)*

### 3.6 Port
*(pending)*

### 3.7 Attestations: SLSA, in-toto, Sigstore, Tekton Chains, GitHub and Azure signing
*(pending)*

### 3.8 Others checked
*(pending)*

## 4. Side by side

*(pending)*

## 5. Recommendation

*(pending)*

## 6. Calls only the owner can make

*(pending)*

## 7. What this study did not verify

*(pending)*
