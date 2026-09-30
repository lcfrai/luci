# 00 - What Panacea already provides (inventory for PanaceaApp)

**Status:** IN PROGRESS (skeleton written first; sections fill in one at a time).
**Date:** 2026-09-30. **Cluster:** 00-panacea. **Feeds:** every other cluster (01-06) and the plan.
**Scope:** a read-only inventory of every Panacea capability PanaceaApp can call today instead of
building or buying it: the entry point (panacea-api route with file:line, or panacea-mcp tool),
what it does, how a caller from another product authenticates, and its gaps for our use. Then the
candidates side by side (Panacea, Coolify + Supabase with Panacea snippets, others) and ONE
recommendation.

**Direction this was written under (2026-09-30, owner):** "focus on using third party solutions ...
I need this tool ASAP, not in weeks"; Panacea is the strong default and the best fully-fledged
solution wins ("Panacea is fine to use majority of, if we want to use Coolify plus Supabase, and
enhance with snippets from Panacea, that is fine as well"). OD-13 relaxed: Panacea snippets may be
copied with attribution in the file header. Orca is the cockpit pick (not re-argued here).

**Method:** grep only, scoped per directory, in `Projects/Panacea` (read-only; nothing built, run
or written there). Every claim below cites `path:line` in that repo as of today's working tree.

---

## 1. Answer first

_(pending)_

## 2. The inventory at a glance

_(pending)_

## 3. Deploy, per-branch previews, promote, rollback, versions and branches

All routes below are in `services/panacea-api/routes/routes.go` (Go 1.22 `ServeMux` patterns) on
the API host `https://api.<domain>` (dev: `api.dev.panacea.kpmg`, `services/panacea-mcp/README.md:47`).
`P` = `/api/v1/projects/{name}`. Role gates are the wrappers at `routes.go:3226-3355`: `authedProject`
= any member (viewer up), `authedProjectDeveloper` = developer up, `authedProjectOwner` = owner.
Every wrapper runs `apiKeyMW` then `authMW` then the RBAC gate then `middleware.Audit` (a
tamper-evident audit row per call). 06-deploy section 2 covers the same ground for phase 7; this
table is the complete list, with the lines.

| Capability | Entry point (method, path, `routes.go` line) | What it does | Gate |
|---|---|---|---|
| Create a project | `POST /api/v1/projects` `:1017`; body `models/models.go:639-663` | namespace + ResourceQuota + ingress + TLS + DNS; takes `repoUrl`, `gitProvider` (GitHub or ADO), `framework`, `defaultBranch`, `team`, `resourceTier`, `enableDatabase`, `enableStorage`, billing `jobCode` | any caller with a genuine grant (`authedGrant`, `:3252`) |
| Connect a repo | `POST /api/v1/git/connect` `:3013`, `GET /api/v1/git/repos` `:3014`, framework detect `:3015`; PATs `:3019-3027` | lists repos, detects the framework, stores a git PAT server-side | user (PAT routes refuse scoped tokens, `authedUnscoped`) |
| Push code and deploy | `POST P/code` `:1033`; `handlers/code_push.go:19-57` | "push a set of files to the project's repo as a single commit and (unless skipDeploy=true) kick off a build/deploy. Designed for programmatic callers ... that hold a pnc_ API token"; limits 1 MiB a file, 10 MiB, 500 files | developer |
| Deploy a branch | `POST P/deploy` `:1286` (body `branch`, `noCache`) | starts the Tekton PipelineRun (build, scans, deploy) | developer + section-writable policy |
| Watch a deploy | `GET P/pipelines/{run}` `:1290`; logs `:2288`; SSE log stream `:2283`; cancel `:1291`; AI failure analysis `:2291`; readiness SSE `P/environments/{env}/readiness/stream` `:2286` | status, step, logs, live stream | member (cancel, analyse: developer) |
| Versions (history) | `GET P/deployments` `:1024`, `GET P/deploys/latest` `:1025`; row shape `models/models.go:665-700` | every deploy with `version` (image tag or short SHA), `commit`, `branch`, `environment`, `failedStep`, `provisioning` | member |
| Rollback | `POST P/deployments/{id}/rollback` `:1026`; `handlers/projects.go:1547` | flips to an earlier immutable Deployment; reports `rollbackWarmth` (warm or cold) | developer |
| Redeploy | `POST P/environments/{env}/redeploy` `:1028` | re-runs the current build | developer |
| Promote (env to env) | `POST P/promote` `:1027`, body `{from,to}`; `handlers/projects.go:1643-1690` | cross-environment promotion; team budget gate and the `allow-prod-autodeploy` capability gate fire for production | developer |
| Promote (stage ladder) | `GET/PUT P/envs/{env}/stage` `:2814`, `:2816`; `POST .../stage/promote` `:2818`; `POST .../stage/approve` `:2860` (owner) | a staged promotion with an owner approval step | developer; approve = owner |
| Branches | `GET P/branches` `:1170`; archive, unarchive, set-primary, pause, resume `:1299-1304`; delete `:1302` (owner) | branch environments as first-class objects | developer (delete: owner) |
| Per-branch previews | `GET P/previews` `:1162`; `GET/PUT P/preview-policy` `:1163-1164`; per-branch auto-deploy `:1308-1309`; URL prefix `:1313-1314`; per-branch tier `:2459` | a live environment per branch, with a policy for which branches get one | member; policy: developer; auto-deploy: owner |
| Preview access | `P/branches/{branch}/security` `:1424-1426` (SSO, IP allow-list), `.../access` `:1430-1431`; share links `:1440-1442` | who can open a preview: SSO, IP list, share link | developer |
| Preview comments | `P/comments` `:2931-2939`; widget `GET /widget/preview-comments.js` `:1013` | reviewers comment on the running preview | member |
| Environments | `GET P/environments` `:1126`; env vars `P/env/{environment}` `:1400-1403`; pods, events, diagnose, quota `:1148-1155` | per-env config and introspection | member; writes: developer |
| Release evidence | `GET P/releases/{release}/evidence` `:2720` | the per-release evidence export (06-deploy N4) | member |
| Gate on scans | `P/envs/{env}/security-posture/{action}` `:2742-2747`; `.../policy-constraints/{action}` `:2767-2769` | the posture engine that blocks or asks acknowledgement per check | owner to set |
| Agent docs | `GET P/agent-context` `:1086` (+ `/version`, `/bootstrap`); `GET P/skill-md` `:1072-1073`; `GET /api/v1/docs/agents` `:642` | machine-readable "how to deploy here" for a coding agent | member |
| Webhook | `POST/DELETE P/webhook` `:2279-2280` | git push to auto-deploy | developer |
| Custom domain | `P/custom-domain` `:1249-1252` | bring a domain, verify it | developer |

**Gaps for our use.**
- **No brief.** A deploy row knows commit, image, branch and env; nothing records the Define artifact.
  06-deploy's glue closes it by writing the brief reference into the commit trailer and the Hub.
- **Promote is two models.** `P/promote` (env to env) and the stage ladder (`P/envs/{env}/stage/*`)
  coexist; which one the cockpit drives is a choice we make (the stage ladder has the approval step).
- **Project = repo = namespace.** One Panacea project per app. PanaceaApp's "project" maps onto it
  one to one, which is what we want, but a project name is also a DNS label (length and charset).
- **Callers from another product need a Panacea identity** (section 5): today a `pnc_` token.


## 4. panacea-mcp: the tools

**Two MCP surfaces exist. Only one works for us today.**

**4.1 `services/panacea-mcp` (stdio, working).** A TypeScript MCP server (`src/index.ts`, 645 lines)
that "holds no permission logic of its own. Every tool forwards your bearer token and the API
enforces all RBAC" (`README.md:5-6`). It authenticates with a `pnc_` token in `PANACEA_TOKEN` and
the base URL in `PANACEA_API_URL` (`README.md:122-127`). Distribution: a Docker image (`README.md:39-65`)
or a single-file bundle from `GET /api/v1/mcp/download` (`routes.go:3010`, `mcpbundle/panacea-mcp.mjs`),
which carries no credential (`README.md:93-94`). Mutating tools require `confirm: true`
(`src/index.ts:402`). **31 tools**, grouped in the source (`group:` field), names at `src/index.ts:249-578`:

| Group (count) | Tools | Who can use them |
|---|---|---|
| read (17) | `panacea_me`, `panacea_projects_list`, `panacea_project_get`, `panacea_deploys_list`, `panacea_deploys_latest`, `panacea_env_list`, `panacea_logs`, `panacea_pods_list`, `panacea_pod_describe`, `panacea_events`, `panacea_diagnose`, `panacea_quota`, `panacea_observability`, `panacea_health`, `panacea_security_status`, `panacea_databases_list`, `panacea_buckets_list` | any token; results auto-scope to the token's projects |
| action (4) | `panacea_deploy` (`:404`, `POST P/deploy` with `branch`, `noCache`), `panacea_redeploy` (`:425`), `panacea_rollback` (`:439`), `panacea_env_set` (`:453`, with `sealed` for write-only secrets) | developer on the project; `confirm: true` |
| admin (3) | `panacea_admin_projects`, `panacea_admin_metrics`, `panacea_admin_audit` | SuperAdmin token only |
| kubectl (6) | `panacea_k8s_namespaces`, `_pods`, `_describe_pod`, `_pod_logs`, `_events`, `_quota` | SuperAdmin, read-only |
| escape (1) | `panacea_api_request` (`:578`): any method and path the token allows; writes need `confirm` | the token's RBAC |

**What it means for PanaceaApp.** Put `panacea-mcp` in every agent sandbox, launched with a
**project-scoped, developer-capped, expiring `pnc_` token** (section 5), and an unmodified Claude
Code can deploy, read logs, diagnose a failed build and roll back with no glue at all. There is no
named tool for promote, previews, branches, code push or evidence; `panacea_api_request` reaches
all of them (`P/promote`, `P/previews`, `P/code`, `P/releases/{release}/evidence`).

**Gaps.** stdio only, so it runs inside the sandbox, not in our control plane; `confirm: true` is
a speed bump the agent passes itself, so the real control is the token's scope and Panacea's
posture gate; the escape hatch means "what the agent can do" is "what the token can do", which is
the right model but must be sized by the token, not the tool list.

**4.2 The API's own remote MCP endpoint (HERALD, A11; not usable yet).** `POST https://api.<domain>/mcp`
(`routes/mcp_wiring.go:34-64`), an OAuth 2.1 protected resource with RFC 9728 metadata at
`/.well-known/oauth-protected-resource`, authorisation server = the Keycloak realm, audience =
the `/mcp` URI, scopes `mcp:discover` < `mcp:read` < `mcp:deploy` < `mcp:db:write` < `mcp:admin`
(`mcp/catalogue.go:13-30`), every call audited with an argument digest (`mcp_wiring.go:225-254`),
gated actions cleared by the same posture engine as a git push (`:66-80`). **But** only
`mcp:discover` and `mcp:read` are published (`catalogue.go:35`), the catalogue holds three tools
(`search_tools`, `describe_tool`, `list_projects`, `catalogue.go:61-90`), and the valid-token path
needs manual client registration in Keycloak, labelled "GATED ... not LIVE" (`mcp_wiring.go:108-113`).
It is the right long-term shape (a remote MCP our cockpit or a team agent calls with the user's own
Keycloak token), and it is not there yet. One inconsistency to check: it names the authorisation
server `https://sso.<domain>/realms/panacea` (`mcp_wiring.go:41`), while Keycloak's hostname is
`https://auth.<domain>` (`deploy/main.tf:963-964`, via `.superpowers/sdd/identity/facts-panacea-keycloak.md` section 1).


## 5. Identity: the Keycloak realm, Entra brokering, and how another product calls in

The realm was surveyed in full on 2026-09-25 (`.superpowers/sdd/identity/facts-panacea-keycloak.md`)
and ruled on in `2026-09-25-identity-keycloak-broker.md`. This section keeps only what an
inventory needs and adds the token facts that survey did not cover.

**5.1 The realm (usable now, one change needed).**
- Realm `panacea` at `https://auth.<domain>/realms/panacea`, Keycloak 25.0.0 from chart
  `codecentric/keycloakx` 2.4.4 (facts section 1). Defined imperatively by
  `deploy/portal/setup-keycloak.sh` (`REALM="panacea"` at `:15`), re-run every deploy.
- Clients: `backstage` (`:139`) and `panacea-oauth2-proxy` (`:320`), both confidential. **No public
  PKCE client exists**; our desktop client `panacea-app-desktop` is one new block in that script
  (facts section 9, ruling K1). It adds no Entra redirect URI, so **no IT ticket** (facts section 10).
- Entra is brokered through IdP alias `entra` (`setup-keycloak.sh:395`), tenant-pinned with a `tid`
  claim filter (`:509`), auto-link on first login, group mappers for `platform-admins` and
  `panacea-admins` (facts section 3). `kc_idp_hint=entra` skips the Keycloak page (facts section 8).
- **Gaps:** one replica with in-memory sessions (`deploy/main.tf:928`, `:954-957`), so a pod move
  logs every user out; a shared blast radius with Panacea; and every realm change is a Panacea deploy.

**5.2 How another product authenticates to panacea-api today** (`middleware/auth.go:130-262`,
`middleware/api_key.go:19-350`).

| Path | What the caller sends | Who it is for | Usable by PanaceaApp? |
|---|---|---|---|
| `pnc_` user token | `Authorization: Bearer pnc_...` (`api_key.go:19`) | programmatic callers, agents, the MCP server | **yes, the path to use** |
| Backstage proxy | `Bearer <BACKEND_SECRET>` + `X-Backstage-Identity: <JWT>`, JWKS-verified, fail closed (`auth.go:156-186`) | the Panacea portal only | no: it is Panacea's shared internal secret |
| Header assertion | `Bearer <BACKEND_SECRET>` + `X-Auth-Request-User/-Email/-Groups` (`auth.go:197-215`) | oauth2-proxy and platform scripts | no, same secret; logged at Warn as a forgeable path |
| Legacy project key | `Bearer panacea_...` from `POST P/api-keys` (`routes.go:2942`, `handlers/apikeys.go:31-55`) | old partner scripts | avoid: expiry in days, `0` = never |
| A realm access token | `Bearer <Keycloak JWT>` | us, ideally | **no**: 401 on REST (`auth.go:245-251`); one JWKS issuer at a time elsewhere (facts section 7) |

**5.3 The `pnc_` token model** (`handlers/api_keys.go`, `middleware/api_key.go`).
- Minted by `POST /api/v1/me/api-keys` (`routes.go:3050`), listed and revoked at `:3051-3052`, admin
  revoke-all at `:3056`. Body: `name`, `expiresIn` (Go duration), and optional `scopeProject` plus
  `scopeRole` = `read` or `deploy` (`api_keys.go:33-39`).
- **Every token expires**: default 90 days, ceiling 365, refused above it (`api_keys.go:71-100`).
- A scoped token is confined to one project at a capped role; the owner's live role is re-checked
  on each request, so a token dies when its owner loses the project (`api_key.go:246-330`).
- **A token cannot mint a token**: "API tokens cannot bootstrap new tokens — must be SSO-logged-in"
  (`api_keys.go:112-116`). And minting needs a real grant (`RequirePortalGrant`, `rbac.go:357-386`).

**Consequence.** PanaceaApp cannot, by itself, give each sandbox its own short-lived,
project-scoped Panacea token: the minting call needs the user's SSO session in Panacea's portal.
Three ways through, ranked:
1. **Ask Panacea for one change: trust our realm client's access token** (audience `panacea-api`)
   as a second issuer. Then the control plane, holding the user's realm token, mints a
   `scopeProject` + `scopeRole=deploy` + `expiresIn=8h` token per sandbox and revokes it at session
   end. The ruling listed this as "not asked of Panacea now" (`2026-09-25-identity-keycloak-broker.md:430-432`);
   under the second direction it is the one Panacea-side change worth asking for. About 1-2 days
   in Panacea (a second verifier config in `jwks.go`, plus tests), done by the Panacea stream, not us.
2. **Until then: one pasted `pnc_` per user, and a control-plane proxy.** The user mints a token once
   in the portal (Account, Connect via MCP); phase 7 already stores it (7P-1, ruling section 8). The
   sandbox's `panacea-mcp` points at a loopback proxy in our control plane that forwards only the
   session's project routes. About 1 day of glue.
3. Not recommended: minting legacy `panacea_` project keys per session (`apikeys.go:31-55`).


## 6. AI: the LiteLLM gateway and its token model

**What runs.** LiteLLM in namespace `panacea-litellm`, service `litellm-proxy:4000`
(`kubernetes/base/network-policies/allow-vvah-to-gateway.yaml:9`), deployed as the Argo CD app
`kubernetes/argocd-apps/ai/litellm.yaml`. Version is ambiguous in the source: the Helm values pin
`ghcr.io/berriai/litellm` `v1.63.11` (`kubernetes/helm-values/litellm-values.yaml:30-31`) while the
custom image builds from `litellm-database:main-v1.40.0` (`deploy/images/litellm/Dockerfile:25`,
`deploy/cloud-shell-deploy.sh:692`). Which one is live is unverified.

**Upstreams.** Claude goes to **Kelios**, KPMG's internal endpoint `api.kelios.kpmg`, which holds
the Anthropic credential (`deploy/set-kelios.sh:3-13`; `deploy/panacea-deploy.sh:2219`); GPT models go
to Azure OpenAI or AI Foundry (`litellm-values.yaml:134-164`). **Bedrock was removed** from the
default providers (`handlers/ai_settings.go:218`).

**The token model: per project, never per user or per session.**
- A LiteLLM **virtual key per project**, `team_id == project`, minted with the master key on first
  use and persisted (`handlers/llm_vkeys.go:209` `generateLiteLLMKey`, `:458` `EnsureProjectLLMKey`),
  with a model allow-list (`:79`) and rate caps (`:263-340`); optionally **per (project, env)** for
  branch environments, still billing to the project (`clients/interfaces.go:758-778`).
- Injected into the tenant app at deploy as `OPENAI_*` and `ANTHROPIC_*` env vars; the master key
  and the Kelios key never leave LiteLLM (`llm_vkeys.go:777-797`).
- A second mode routes through **panacea-api's own metered proxy**, `/ai/proxy/{project}/{mode}/v1/...`
  (`routes.go:1763-1764`), with a per-project proxy token (`llm_vkeys.go:736-763`); fleet-wide switch
  at `POST /api/v1/admin/llm/migrate-keys` (`routes.go:2094`).
- API for a project: model `GET/PUT P/llm/model` (`routes.go:2391-2392`), usage `P/llm/usage`
  (`:2389`), calls `P/ai-calls` (`:1540`), rate `P/llm-rate` (`:2871`), key rotate
  `POST P/ai/key/regenerate` (`:2403`). A fail-closed AI governance pre-screen (PII redaction and
  prompt-injection blocking) runs locally; external moderation is off by default (`STATUS.md:142-150`).

**Gaps for our agents (why we do not route Claude Code through it).**
1. **Both endpoints are in-cluster** (`litellm-proxy:4000`, `panacea-api.panacea-portal.svc.cluster.local`,
   `llm_vkeys.go:744`). Our sandboxes run in our own environment (`OD-5`), so they cannot reach it.
2. **Attribution stops at the project**: `team_id == project` and `user == project`
   (`handlers/deploy_analysis.go:30-31`). `OD-19` needs user, team and project, per session.
3. **No AU path**: `OD-19` AU = Bedrock Sydney; Panacea removed Bedrock. Global = the direct
   Anthropic API; Panacea's Claude upstream is Kelios, whose retention terms are not in the repo.
4. **Zero data retention is not asserted in config**: no `turn_off_message_logging` or
   `store_prompts_in_spend_logs` setting exists in `deploy/` or `kubernetes/` (grep, 0 hits), so it
   rests on LiteLLM's defaults and Kelios's terms (unverified).

**Verdict.** Run our own LiteLLM (MIT core; phase 6, `OD-21`) and **copy Panacea's key-minting,
allow-list and rate-cap code** into our broker, with attribution (`OD-13` relaxed). Use Panacea's
gateway only for the apps Panacea hosts, where it already works with no glue.


## 7. Platform services: Backstage, Vault, Postgres, object storage

**Panacea is already a Supabase-class backend for the apps it hosts.** Per project it offers a
database, a REST data API, realtime, functions, storage, app users, flags and jobs, each behind the
same RBAC and audit as the deploy routes. That matters for the Coolify-plus-Supabase question in
section 10: for the apps PanaceaApp ships, Panacea already *is* that pair, plus Entra SSO and scans.

| Service | Entry point | What it holds or does | Licence of the component | Gaps for us |
|---|---|---|---|---|
| **Backstage** (the Panacea Portal) | `backstage/`; OIDC to Keycloak (`backstage/app-config.production.yaml:36-47`); proxies `panacea-api` (`:54-93`) | the web portal; custom plugins `agent` (Forge), `pages`, `risk`, `team` (`packages/app/src/plugins/`); the catalog holds **only 8 scaffolder templates** (`app-config.production.yaml:103-139`: nextjs-postgres, python-fastapi, node-express, rag-pipeline, ai-agent, static-site, static-html, multi-container; `templates/`), plus TechDocs | Apache-2.0 | no tenant Components in the catalog; a Backstage-issued identity we cannot mint. Use by **deep link** (06-deploy 3.5) and take the 8 templates as Develop starters |
| **Vault** | indirect: env vars `P/env/{environment}` (`routes.go:1400-1403`, `sealed` flag); connections `P/connections` (`:2660-2663`) | KV v2 at `secret/data/tenants/<project>/<env>` and connection secrets at platform, team and tenant scope (`clients/vault.go:308-345`); Transit for release signing (`clients/vault_transit.go`); chart 0.28.1 (`kubernetes/argocd-apps/platform/vault.yaml:21-22`) | **BUSL-1.1** (restricted) | Shamir-unsealed; "Vault reliability" is on Panacea's owner list today (Panacea commit `efc637879`). Not our secrets store: ours is Azure Key Vault (06-platform study) |
| **Postgres** | `POST /api/v1/databases` (`routes.go:1325`), per-project `P/services/database` (`:1319`); SQL `POST /api/v1/databases/{name}/query` (`:2295`); schema `P/db/schema`, `P/db/types` (`:2321-2322`) | CloudNativePG chart 0.21.2 (`argocd-apps/data/cloudnativepg.yaml:22-23`); shared or dedicated (`models/models.go:651`, upgrade `routes.go:1347`); backups, schedule, restore, PITR revert (`:1330-1357`); **copy-on-write DB branch per preview** (`clients/db_branch_ops.go:1-13`); pooling, HA, extensions incl. pgvector (`clients/db_*_ops.go`) | Apache-2.0 | none for tenant apps; it is in Panacea's cluster, so it is not our Hub's database unless the Hub is a Panacea project (section 10) |
| **Data API, realtime** | `P/rest/{table}` (`routes.go:2915-2918`), `P/data-api` (`:1469-1470`); `P/realtime` connect, publish (`:2911-2912`) | PostgREST-style CRUD with row-level security, and a pub/sub channel | Panacea code | Panacea-shaped, not Supabase-API compatible |
| **Object storage** | `/api/v1/buckets` (`routes.go:1361-1372`), presign (`:1367`), virus scan (`:1369`); per project `P/storage/{bucket}/*` incl. backup, snapshots, restore (`:2335-2344`) | MinIO chart 5.2.0 (`argocd-apps/data/minio.yaml:22-23`) behind `s3proxy`; ClamAV scan (`argocd-apps/app-services/clamav.yaml`) | **MinIO AGPL-3.0, and the upstream repo is archived** (GitHub API, 2026-09-30: `archived: true`); ClamAV GPL-2.0 | an unmaintained core dependency on Panacea's side; for our own storage use Azure Blob |
| **Functions, app users, flags, jobs** | `P/functions` (`routes.go:2905-2908`, Knative Serving 1.13.0); `P/auth/users` (`:2947-2956`); `P/flags` (`:2415-2419`, Flagsmith); `P/jobs` (`:1261-1266`) | serverless functions, the app's own users in Keycloak, feature flags, cron jobs | Knative Apache-2.0; Flagsmith BSD-3 | app users are Keycloak users; a per-app OIDC client is still manual (`LAUNCH.md:1784`, `:1939`) |
| **Also in the estate** | Argo CD apps under `kubernetes/argocd-apps/` | Postal (email), Gotenberg (PDF), Presidio 2.2.355 (PII), Redis chart 19.5.0, KEDA, Traefik, cert-manager, Prometheus, Grafana, Loki, Tempo, Uptime Kuma | mostly permissive; **Grafana, Loki, Tempo are AGPL-3.0** | licence flags only matter if this stack is ever shipped into a customer tenancy |

**Versions.** API v1.163.0 and Portal v1.142.0 (`VERSIONS.md`, last changed 2026-09-14);
`STATUS.md` is a snapshot last changed 2026-08-10 and already lags the code (it says there is no
per-key regenerate route; `routes.go:2403` has one), so this study cites code over status files.


## 8. Builds and security: Tekton, Trivy, Falco and the other scanners

**We never call Tekton directly.** `POST P/deploy` (or `P/code`, a git push through the webhook,
or `P/promote`) starts it; we watch it through `P/pipelines/{run}` and its log stream (section 3).

**The pipeline** (`pipelines/pipelines/build-deploy.yaml`, 2,517 lines; tasks at `:295-1685`, `finally`
at `:1685-2503`): cancel-superseded, clone, detect, **secret-scan, sast-scan, iac-scan, sca-scan,
model-scan**, risk-assess, build-plan, build (Cloud Native Buildpacks or Kaniko; `build-engine`
"classic" or "buildkit", `:109-112`), **trivy-scan** (image), **license-scan**, db-migrate,
data-api-declare, db-posture-scan, **sign-release**, deploy, deploy-checks, **dast-scan, nuclei-scan,
schemathesis-scan**, report-live; then `record-gate-results`, **`mint-release`**, `launch-advisory`,
report-failure, cleanup. Previews run `preview-deploy.yaml` (1,240 lines) and `preview-cleanup.yaml`.
Triggers: a Tekton EventListener with GitHub push and PR triggers (`pipelines/triggers/`) and the
`webhook-receiver` service for Azure DevOps and GitHub.

**The scanners** (image pins from `pipelines/tasks/*-scan.yaml`, mirrored into Panacea's ACR):

| Check | Tool and pin | Licence | Note |
|---|---|---|---|
| secrets | gitleaks v8.21.2 | MIT | |
| SAST | Semgrep 1.86.0, `--config auto` (`sast-scan.yaml:118-123`) | engine LGPL-2.1; **registry rules under the Semgrep Rules License v1.0**, "internal business purposes" only, no offering as a service | fine inside KPMG; a blocker if PanaceaApp is sold with these rules. `--config auto` also fetches rules from semgrep.dev (the task's own sovereignty note, `:27-31`) |
| SCA | OSV-Scanner v2.0.2 | Apache-2.0 | malicious-package detection |
| IaC, licence, image CVE | Trivy 0.58.0 (+ cosign 2.4.1) | Apache-2.0 | |
| models | modelscan 0.8.5 | Apache-2.0 | |
| DAST | ZAP (weekly 2025-01), Nuclei v3.3.7, Schemathesis v3.39.5 | Apache-2.0, MIT, MIT | run against the deployed env |
| DB posture | Panacea's own build image | Panacea | |
| agentic SAST (VVAH) | `vvaharness:v1.1.0` | Panacea | owner notes say retired 2026-09-23 (not checked in code) |
| uploads | ClamAV | GPL-2.0 | bucket virus scan |
| runtime | Falco chart 4.17.2 (`argocd-apps/platform/falco.yaml`), trivy-operator, Kyverno 3.2.6 (signatures in **Audit**), Gatekeeper 3.17.1 | Apache-2.0 | watches the node kernel |

**The gate.** Each scan has a tri-state mode (off, warn, block), and the default is warn
(`build-deploy.yaml:413`). The posture engine (`handlers/security_policy.go`) is a platform baseline
that teams and projects can tighten but not loosen, per check `block` or `ack`, fail-closed, gating
**prod-deploy, serve-without-SSO, external-domain and apothecary-publish** (`STATUS.md:125-133`). Our
entry points: run scans `POST P/scans/run` (owner, `routes.go:2750`), results `P/security-status`
and `/{scan}` (`:1043-1044`), runtime `P/runtime-security` (`:1066`), posture and constraints
(`:2742-2769`), risk assessment (`:1175-1239`), evidence (`:2720`).

**This is `OD-24` already built.** "What security scans are needed from a deployment stand point"
is the posture engine plus a data classification that picks the policy. PanaceaApp sets the
classification and reads the verdicts; it does not run a scanner.

**Gaps.** Scans default to warn, so a strict gate is a policy we set per project (owner role).
`sign-release` was "not yet wired" at 2026-08-10 (`STATUS.md:135-137`) but is a task in the pipeline
today (`build-deploy.yaml:1234`); whether dev signs now is unverified. Falco sees the node kernel,
so it cannot see inside a Kata guest (01-sandboxes 4.1).


## 9. Isolation: tenant namespaces, and Forge (agent loop and hardened preview pods)

**9.1 Tenant namespace isolation** (`services/panacea-api/clients/tenant_ensure.go`, 1,011 lines;
`clients/kubernetes.go`; `clients/pod_security.go`). One namespace per project environment, created
by `POST /api/v1/projects` and by each branch deploy. What it guarantees:
- **Pod Security Admission** `enforce=baseline`, `warn`/`audit=restricted`, stamped last so a caller's
  labels cannot lower it (`pod_security.go:58-69`, `kubernetes.go:839-843`).
- **NetworkPolicy baseline** (ISM-1234): default-deny-ingress, allow same namespace, allow Traefik on
  the app ports, and `deny-metadata-egress` (all egress except link-local, so no IMDS)
  (`tenant_ensure.go:20-63`, `:83-160`). **Egress is deliberately open at the pod layer** and governed
  by the Azure Firewall route (`:45-50`, `:103-107`), after a deny-egress baseline broke every app's
  database access (`:38-44`).
- ResourceQuota per namespace; a sweeper that re-asserts drifted tenants and deletes stale branch
  namespaces (`SweepEnsureTenants` `:577`, `sweepStaleBranchNamespaces` `:919`).
- Runtime: `runc` on shared nodes (no RuntimeClass anywhere in the tenant path), Falco on the node.

**For us:** the right isolation for the *apps we ship*, and a baseline worth copying for our own
cluster (01-sandboxes 4.15 already takes it). It is not agent-sandbox isolation: shared kernel,
open pod egress.

**9.2 Forge, the agentic build loop** (`services/forge-orchestrator/`). Panacea's own in-house
agent: an LLM tool loop (`orchestrator/orchestrator.go`, bounded tool round trips) that writes files,
runs vetted commands in a preview pod, and must clear verify gates before it may ship: typecheck,
build, boot, test (`orchestrator/verify.go`), **authorisation-first** (`authz_gate.go:3-62`: every route
the code serves must be declared, and the probe is regenerated each turn), a browser test
(`browsertest/`), design and contrast repair (`repair/`), scaffolds (`scaffold/`), and **ship** to
Gitea or GitHub then trigger the Panacea build (`ship/`). Sessions have create, fork, cancel and
visibility (`forge-orchestrator/routes/*.go:77-109`); the model is reached through `AI_PROVIDER_URL`
(Kelios or Anthropic, `deploy/forge/deploy-forge-orchestrator.yaml:303-329`).
- **Access:** only through `panacea-api` at `/api/v1/forge/*` (`routes.go:3066-3126`) behind
  `authedAgent`: the agent-mode platform flag ON, a **platform-admin** caller, and **not an API key**
  (`routes.go:3261-3273`, `middleware/agent_gate.go:41`). A product like ours cannot drive it.
- **Overlap with us:** Forge is a closed agent loop; PanaceaApp runs unmodified Claude Code. We do
  not adopt the loop. We **copy its gates** as checks our agents run before a session may ship
  (authorisation-first, verify, browser test), with attribution.

**9.3 Forge's hardened preview pods: could they host our agent sandboxes? No, but copy the spec.**
`preview/manager.go:81-223`: one pod per session, UID 1001, `runAsNonRoot`, read-only root,
`allowPrivilegeEscalation:false`, drop ALL capabilities, seccomp `RuntimeDefault`, `emptyDir` for
`/tmp`, source and `node_modules`, 100m/256Mi requests and 500m/1Gi limits, a `maxPods` cap
(`:82-88`) and TTL cleanup; **deny-all egress except DNS and the in-cluster npm and PyPI mirrors**
(`preview-deny-egress`, `deploy/forge/deploy-forge-orchestrator.yaml:49-101`; Verdaccio 4873, proxpi
5000, `manager.go:144-156`). Why not host Claude Code there:
1. `runc`, shared kernel; the plan and 01-sandboxes require Kata microVMs for untrusted agents.
2. Egress is closed to the model gateway and the git host, which an agent session needs.
3. The image is a Next/Vite preview server; no PTY, no worktree, no warm pool, no persistent disk.
4. It sits in Panacea's cluster, behind a platform-admin-only gate, in another session's repo.

**Verdict, consistent with 01-sandboxes 4.15:** integrate the code (pod security context, deny-egress
policy, package mirrors, tenant baseline), reject Panacea as the host of our sandboxes.


## 10. Candidates side by side

**The question.** Which whole product gives PanaceaApp the platform layer: somewhere to ship each
app (deploy, previews, promote, rollback, scans, evidence), the app's backend (database, storage,
secrets, AI key), sign-in, and a backend for our own shared state? Panacea is the strong default;
the owner named Coolify plus Supabase, with Panacea snippets, as the allowed alternative.

| Candidate | Licence | Deploys where | Enterprise fit (Entra SSO, audit, AU) | Maturity | Days | Verdict |
|---|---|---|---|---|---|---|
| **C1 Panacea**, API + `panacea-mcp` + the realm + snippets | KPMG's own code (its stack holds BUSL Vault and AGPL MinIO, Grafana, Loki, Tempo) | runs today on KPMG's Azure, `australiacentral` (`deploy/terraform.tfvars:25`), an Australia East rebuild in cutover | Entra via Keycloak; hash-chained audit on every route; AU yes; posture-gated scans | API v1.163.0, Portal v1.142.0 (`VERSIONS.md`, 2026-09-14); live dev estate | **2** here (section 12), beyond 06-deploy's 6 | **adopt** |
| **C2 Coolify + Supabase**, self-hosted, with Panacea snippets | both Apache-2.0; Supabase Auth MIT (GitHub API, 2026-09-30) | Coolify: Docker on Azure VMs, no AKS; Supabase: Docker Compose or a community Helm chart | Coolify OIDC only in a 4.4 RC (06-deploy 3.1), stable is v4.3.23 (2026-09-18); no Azure DevOps (fails `OD-4`); self-hosted Supabase is **one project per stack**, with no branching, managed PITR or management API ([Supabase docs](https://supabase.com/docs/guides/self-hosting)) | 62.4k and 110.9k stars; both active this week | 16-24 to approach C1 (06-deploy section 5) | **reject** for shipped apps |
| **C3 Azure-native**: PostgreSQL Flexible Server, Blob, Key Vault, Container Apps or AKS | commercial (Azure consumption) | our subscription, Australia East | Entra-native, private endpoints, Defender, PITR, zone-redundant HA, AU yes | GA services | already in phase 8 (tasks 05, 08) | **buy**, for our own backend only |
| **C4 Supabase for the Hub store** | Apache-2.0 | self-hosted on Azure, or Supabase Cloud in Sydney (AWS, fails `OD-18`) | REST, realtime and storage on day one, but its Auth duplicates Keycloak and self-hosted backups are ours | as C2 | 3-4 to replace the Hub's Postgres layer | **reference** |
| **C5 Appwrite** (GLASSES `6968183a54d9012013f1`) | BSD-3-Clause | self-hosted Docker | no Entra for its console; no scans, gate or evidence | 57.5k stars | not costed | **reject** |
| **C6 The Hub as a Panacea project** | as C1 | Panacea's cluster | as C1, plus our shared state on another team's dev estate | as C1 | 0.5 | **reject** as the default (conflicts with `OD-5`) |

**C1 on merit.** It is the only candidate that already does the whole shipped-app job in our
regulated setting: per-branch previews with SSO and a branched database, a stage ladder with owner
approval, rollback with warmth, 18 scan checks behind a monotone posture gate, a per-release
evidence export, per-app Postgres, storage, secrets and an AI key, and Azure DevOps plus GitHub.
Everything it lacks for us is small and named in section 12. **Trade-off: our shipped apps share
Panacea's estate and its incidents; in exchange we build almost none of it.**

**C2 on merit.** Coolify plus Supabase is a good small-team pair and a weaker enterprise one. For the
apps we ship, every app would be its own Supabase stack of about a dozen containers, with no
branch-per-preview database, no PITR and no API to create one from the cockpit, behind Coolify's
Docker hosts with no Azure DevOps and no scans. Panacea snippets would have to supply the gate,
scans and evidence: that is re-building Panacea on a weaker base. **Trade-off: permissive and
portable to a customer's cloud, but 16-24 days to approach what C1 gives on day one.**

**C3 and C4, for our own backend.** The phase 8 plan already puts the Hub on Flexible Server,
Entra-only (`env` plan task 08) and secrets in Key Vault (task 05). Supabase would give REST and
realtime for free, but the Hub's contract is its own Go API (`hubapi`) with row-level security
and a hash-chained audit (Hub tasks 07a-14), so Supabase would replace little. **Keep C3.**

**C6, the fastest store and the wrong one.** Creating the Hub as a Panacea project gives Postgres,
storage, TLS and scans in half a day, but puts `OD-3`'s shared state on Panacea's dev estate and
contradicts `OD-5`. Use it only as a stopgap if our subscription is late (section 13).


## 11. Recommendation

**Adopt Panacea (C1) as the platform for everything PanaceaApp ships, and as its sign-in.** Call it
through its REST API with the user's `pnc_` token from the control plane, and through
`panacea-mcp` inside every sandbox (behind 06-deploy's allowlist proxy). Copy its proven code, with
attribution, wherever we must run our own piece. Keep three things off Panacea, on merit.

| Use Panacea for | How |
|---|---|
| Deploy, previews, promote, rollback, versions, branches, evidence | the section 3 routes; 06-deploy's recommendation (about 6 days) |
| Each shipped app's backend: Postgres (branched per preview), storage, secrets, AI key, functions, app users, flags, jobs | automatic at project create and deploy (section 7); nothing to build |
| Scans and the gate `OD-24` asks for | the posture engine and the 18 checks (section 8); we set the class, it runs the scans |
| Sign-in (`OD-30`) and teams | the realm plus one public client (section 5.1); Panacea's team API (05-work-team) |
| Agent know-how | `panacea-mcp`'s 31 tools and `P/skill-md` / `P/agent-context` fed to Claude Code (section 12) |

| Keep off Panacea | Why | Instead |
|---|---|---|
| Agent sandboxes | `runc`, open or closed-to-model egress, admin-only, another team's cluster (section 9.3) | Kata on our AKS (01-sandboxes), hardened with Forge's pod spec, deny-egress policy and mirrors, copied |
| Agent inference | in-cluster only, per-project keys, no Bedrock, Kelios terms unknown (section 6) | our LiteLLM (phase 6), with Panacea's key-minting and rate-cap code copied |
| Our own shared state | `OD-5`; Panacea's dev estate; MinIO archived upstream (section 7) | Flexible Server, Blob and Key Vault in our subscription (phase 8 tasks 05, 08) |

**Integration: 2 days of glue here** (section 12), on top of 06-deploy's 6, plus three small asks
of the Panacea stream (one Keycloak client, one allowlist address, and optionally a second trusted
issuer). **Usable on day 1** for an agent: a sandbox with `panacea-mcp` deploys its branch to a
preview, reads the logs and rolls back.

**What it replaces in our plan.**
- **Phase 7 (20 tasks, 4 built): 8 tasks leave**, counted by 06-deploy (section 5 there).
- **Phase 8 (35 tasks, 5 built): task 09 (our own realm as code, 2.5 h) is deferred**, because sign-in
  stays on Panacea's realm, as the identity ruling already allows until phase 8 (ruling section 9).
  Tasks 11 (platform chart, 3 h) and 14 (sandbox hardening, 2.5 h) are **shortened** by copying
  `tenant_ensure.go`'s baseline, `pod_security.go` and Forge's pod spec.
- **Phase 6 (29 tasks, plans only): none removed; tasks 04, 06 and 09 are shortened** by copying
  `handlers/llm_vkeys.go` (mint, delete, allow-list, rate caps), `budget_sync.go` and the spend-log
  queries in `handlers/costs.go`.
- **Hub (50 tasks, 6 built): none removed here.** Task 14 (the hash-chained write) is shortened by
  `clients/audit_pg.go` and `audit_signed_head.go`; teams (15, 16a) and evidence (25) are 05's and
  06's to count.
- **What never enters the plan:** a shipped-app backend, previews, scans and evidence. C2 would have
  added 16-24 days of it.

**Risks.** Shipped apps share Panacea's estate and incidents, including a Keycloak that logs everyone
out on a pod move (section 5.1). Panacea's edge may be locked to KPMG egress (section 15), so our
AKS egress address must be allowed. MinIO, Panacea's object store, is archived upstream (section 7):
Panacea's problem, and a reason not to put our own state there. Scans default to warn. Panacea's
prod estate is mid-cutover, so we point at dev first, as the identity ruling does.


## 12. The glue we still build

Only what no other cluster counts. 06-deploy's G1-G4 (the Orca Deploy panel, signed release
provenance, promotion and the data class, the agent allowlist proxy) are **not** repeated here.

| # | Glue | Uses | Days |
|---|---|---|---|
| P1 | **`panacea-mcp` in the sandbox image**: the single-file bundle from `GET /api/v1/mcp/download` (`routes.go:3010`), pinned by hash; at session bind write its MCP config with `PANACEA_API_URL` pointed at 06-deploy's G4 proxy, never the user's token | section 4.1 | 0.25 |
| P2 | **Panacea's agent instructions into the worktree**: at bind, write the stable bootstrap skill from `GET P/agent-context/bootstrap` (`routes.go:1088`), which makes Claude Code fetch `GET P/agent-context` (the per-project skill plus Panacea's `LAUNCH.md` deploy contract in one call, "the LIVE-REFRESH mechanism for a Claude Code agent's platform instructions", `handlers/agent_context.go:13-26`); refresh on `/version` (`:1087`) | section 3 | 0.25 |
| P3 | **Create the Panacea project from the cockpit.** Phase 7's binding (task 06) only finds a Panacea project by repository; add create-if-missing: `POST /api/v1/projects` (`routes.go:1017`) with `repoUrl`, `gitProvider`, `team`, `framework`, `defaultBranch`, `jobCode`, `enableDatabase`, `enableStorage` (`models/models.go:639-663`), after registering the user's git PAT once with `POST /api/v1/git/pats` (`routes.go:3020`, unscoped `pnc_` accepted). Without it, "nothing to a shipped app" sends the user to Panacea's portal first | sections 3, 5.3 | 0.5 |
| P4 | **The snippet ledger.** One file listing every Panacea file we copy (source path, Panacea commit, our path), the header every copied file carries (`OD-13` relaxed), and a CI check that the header survives edits | all | 0.25 |
| P5 | **The three Panacea asks, written and then checked**: (a) the `panacea-app-desktop` public PKCE client in `deploy/portal/setup-keycloak.sh` (ruling K1); (b) our AKS egress address in `edge_allowed_ip_ranges` (`deploy/main.tf:580-605`); (c) optional, the realm as a second trusted issuer with audience `panacea-api` (section 5.3), which removes the pasted token and lets us mint an 8-hour project-scoped `pnc_` per sandbox | section 5 | 0.25 |
| P6 | **The live proof on Panacea dev**: one sandbox session creates a project, deploys a branch preview through `panacea-mcp`, opens it through the SSO gate, rolls back; and checks that PR previews still fire when the edge is IP-locked (inbound webhooks stop; `deploy/restrict-edge-allowlist.sh:20-22` moves git push to outbound polling) | sections 3, 8 | 0.5 |
| | **Total** | | **2** |

**Later, optional (1 day):** Forge's ship gates as a check the agent runs before `panacea_deploy`:
the authorisation-first probe and its zero-tests trap (`orchestrator/authz_gate.go:3-62`, `:173-180`)
and the verify stages (`orchestrator/verify.go`). Worth it once agents ship scaffolded apps.

**Snippet ports that live inside existing tasks** (no new days; each shortens its task):

| Panacea source | Into our task | What it saves |
|---|---|---|
| `services/panacea-api/handlers/llm_vkeys.go:209-400`, `handlers/budget_sync.go`, the `/spend/logs/v2` query in `handlers/costs.go:1905` | phase 6 tasks 04, 06, 09 | LiteLLM admin calls, allow-list, rate caps, budget sync, spend by team |
| `deploy/portal/setup-keycloak.sh:113-635` (realm, clients, Entra IdP, first-broker-login, mappers) | phase 8 task 09, when it runs | the realm's proven settings, as the source for our realm-as-code JSON |
| `clients/tenant_ensure.go:20-160`, `clients/pod_security.go:58-69` | phase 8 task 11 | the tenant NetworkPolicy baseline and PSA labels, incident notes included |
| `services/forge-orchestrator/preview/manager.go:92-223`, `deploy/forge/deploy-forge-orchestrator.yaml:49-101` | phase 8 task 14 (01-sandboxes counts it) | pod security context, deny-egress policy, package mirrors |
| `clients/audit_pg.go`, `clients/audit_signed_head.go` | Hub task 14 | the hash-chained audit write and the signed chain-head anchor that detects truncation |
| `services/panacea-build/pkg/prcomment/client.go:54` and the hook lifecycle | 05-work-team | the one live PR comment per session (05 counts it) |

**One naming clash to settle in code.** Phase 6 task 14 plans a sandbox binary called `panacea`
(`panacea token`). Panacea already ships a CLI with that name (`services/panacea-cli/cmd/root.go:56`),
and an agent may be given both. Rename ours before it ships.


## 13. Owner calls

Only money or what customers see. Everything else above is decided.

1. **Customers' apps run on Panacea's estate (customer-visible).** Previews and releases live at
   Panacea hosts (or a custom domain, `routes.go:1249-1252`). If Panacea's edge is locked to KPMG's
   14 corporate egress addresses (`deploy/restrict-edge-allowlist.sh:3-4`), **nobody outside
   KPMG's network can open them**. Fine for internal use and KPMG-hosted demos; a customer reviewing
   a preview from their own office needs Panacea's edge policy to change, per host or per branch.
   Call: is internal-network-only acceptable for the first customers?
2. **Who pays for shipped apps (money).** Panacea bills each project to its owning team's month-end
   statement, tagged with an engagement job code and phase (`models/models.go:652-660`), and meters
   each project's AI spend (section 6). Call: which team and job code own the apps PanaceaApp
   creates (one PanaceaApp team, or the user's own team per engagement)? P3 needs the answer.
3. **A stopgap store, only if our subscription is late (money and `OD-5`).** If the Azure
   subscription for our environment is not ready, the Hub can run as a Panacea project for a while
   (C6, half a day), at the cost of setting `OD-5` aside for our shared state until the move. Our
   recommendation is not to, unless the date forces it.
4. **Later, before any customer-hosted tier (money).** Panacea's stack carries licences that do not
   travel into a customer's tenancy as they are: Semgrep's registry rules are internal-use only,
   Vault is BUSL-1.1, MinIO, Grafana, Loki and Tempo are AGPL-3.0, ClamAV is GPL-2.0 (sections 7-8).
   `OD-32` already swaps Vault for OpenBao at the lift; the others need the same treatment then, or a
   commercial licence (Semgrep is priced per contributor; Grafana Enterprise by quote; neither
   price was checked today).


## 14. Revised for the owner's second direction

**There was no earlier version to revise.** The first run of this research was stopped at 09:47
before it wrote this file (no `00-panacea.md` in `git log --all` before this run's skeleton commit),
so every section was written under the second direction. What that direction changed, against the
position the repository held before today:

| Before (the locked decisions and plans) | Now | Why |
|---|---|---|
| Panacea is **phase 7's API target only**; "copy no Panacea source" (`OD-13`) | Panacea is the **platform for everything shipped** (deploy, previews, the app's backend, scans, evidence), the **sign-in**, and the **team API**; its code is **copied with attribution** into our sandbox, gateway, environment and Hub tasks | the owner: Panacea is the strong default, snippets allowed |
| Phase 8 builds **our own Keycloak realm** (task 09) | **deferred**: we stay on Panacea's realm with one new client | it already brokers Entra with no IT ticket; speed |
| Coolify and Supabase never assessed | assessed on merit (section 10): **rejected** for shipped apps, Supabase a **reference** for the Hub store | the owner allowed them; they lose on ADO, isolation, previews with data, scans and evidence |
| Forge's pod template was a pattern to read | its **code is copied** into our sandbox hardening; Forge itself is **not** used | its pods are `runc` and preview-shaped; its loop is admin-only and not Claude Code |
| Panacea's LiteLLM lessons were read, not copied (phase 6 plan) | its key-minting, allow-list, rate-cap and budget code is **copied**; its gateway is still **not** used for agents | in-cluster only, per-project keys, no Bedrock AU path |

**Unchanged on merit:** agent sandboxes on our own AKS with Kata (01-sandboxes), our own LiteLLM,
and our own shared state on Flexible Server (`OD-5`). Panacea is the better answer for what it
already runs, and not for what it was never built to run. **Orca** (the cockpit pick) is untouched
by this file; 02-fleet-cockpit and 06-deploy place Panacea's previews and Deploy panel inside it.


## 15. What was not verified

_(pending)_

## 16. Sources

_(pending)_
