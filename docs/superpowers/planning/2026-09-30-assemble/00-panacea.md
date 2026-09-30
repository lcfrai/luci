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

_(pending)_

## 5. Identity: the Keycloak realm, Entra brokering, and how another product calls in

_(pending)_

## 6. AI: the LiteLLM gateway and its token model

_(pending)_

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
