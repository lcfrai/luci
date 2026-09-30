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

_(pending)_

## 8. Builds and security: Tekton, Trivy, Falco and the other scanners

_(pending)_

## 9. Isolation: tenant namespaces, and Forge (agent loop and hardened preview pods)

_(pending)_

## 10. Candidates side by side

_(pending)_

## 11. Recommendation

_(pending)_

## 12. The glue we still build

_(pending)_

## 13. Owner calls

_(pending)_

## 14. Revised for the owner's second direction

_(pending)_

## 15. What was not verified

_(pending)_

## 16. Sources

_(pending)_
