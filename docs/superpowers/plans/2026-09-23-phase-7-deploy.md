# Phase 7 — the Deploy connector: implementation plan

> **For agentic workers:** execute task by task with `superpowers:subagent-driven-development` or
> `superpowers:executing-plans`. Every task's part file is in
> `docs/superpowers/plans/2026-09-23-phase-7-deploy-parts/task-NN.md`. Steps use `- [ ]` checkboxes.
> Every task is test-first.

**Status:** written 2026-09-23 on branch `feat/phase-7-deploy` (worktree
`.claude/worktrees/phase-7`, based on `3303fd6`). Owner-reviewable under `OD-1`; adversarially
reviewed before code. **Nothing in this plan was compiled or run.** Every citation below was opened
in this pass unless it is marked *unverified*; §10 lists what was not verified.

---

## 1. Goal

A branch in the cockpit gets a **Ship** action. Ship reserves a release in the Hub (which fixes the
brief pin), starts a real Tekton pipeline on Panacea through `panacea-api`, and confirms the run
against the reservation. The **Deploy lens** then becomes the honest account of what happened, per
environment: which commit, which image, which version, whether the build passed, whether the scans
passed, whether pods are actually serving, and **which brief revision the release was shipped
against**. Cancel, rollback and rebuild (the brief's "redeploy", 7P-3) are the three other acts; auto-deploy, project access,
branch access posture and the URL prefix are the four governance controls, each drawn as the API
really offers it.

It is the last rung of the provenance thread (`CLAUDE.md`, *"attached to the release in Deploy"*):
the pin a release carries is taken by the **Hub** at reservation, never asserted by the caller after
the fact, and never inferred from what the poller observes.

**What is not new:** every Panacea capability used here already exists as a `panacea-api` route
(§4). Phase 7 adds no Panacea endpoint and copies no Panacea source (`OD-13`). It is a typed Go
client behind a `Deployer` interface, a Ship orchestrator, a deploy watcher inside the control
plane, a set of control-plane routes, and the lens.

**Tech stack:** Go 1.23, standard library only for everything phase 7 adds (`net/http`,
`crypto/tls`, `encoding/json`); the Hub's `hubclient`/`hubapi` and `credstore` from the Hub tasks
H0/C1/C2; phase 3's `githost`; React 19 + TypeScript + zustand 5 + vitest, as phase 2 built them.
**No new Go module requirement and no new npm package.**

---

## 2. Architecture

```
 Tauri window (React)            control plane (cmd/api, 127.0.0.1:8730)                 outside
 ┌────────────────────┐  REST   ┌─────────────────────────────────────────────┐
 │ Deploy lens        │ ──────► │ api: /v1/deploy/*, /v1/projects/{id}/deploy/*│
 │  store slice       │ ◄────── │   ship.Orchestrator ──reserve/confirm──────► │──► Hub  /v1/releases…
 │  (not persisted)   │ header  │   deploywatch.Watcher (its own timer)        │     (hubclient.Releases)
 │ poller.ts tick ────┼─────────┤   Panacea-Deploy-Cursor on GET /v1/sessions  │
 └────────────────────┘         │   deploy.Deployer  ◄── the swap seam (OD-13) │
                                │     └ panacea.Client ──── pnc_ bearer ──────►│──► panacea-api /api/v1/…
                                │   credstore.Secrets: PanaceaApp/panacea/<host>│
                                └─────────────────────────────────────────────┘
```

**Four layers, each with one job:**

| Layer | Package | Job | Knows about Panacea? |
|---|---|---|---|
| Domain | `control-plane/internal/deploy` | the `Deployer` interface, the domain types (`Environment`, `Deployment`, `RunProgress`, `ScanReport`, `AccessPosture` …), typed errors, and an in-memory `deploytest.Fake` with a conformance suite | **No.** Nothing in it names a Panacea route, envelope or field spelling |
| Connector | `control-plane/internal/deploy/panacea` | `panacea.Client` implements `deploy.Deployer` over `panacea-api`: transport, auth, error mapping, DTOs, mapping to domain types | **Yes — the only package that does** |
| Orchestration | `control-plane/internal/deploy/ship`, `…/deploywatch`, `…/binding` | Ship/rollback/rebuild sequencing against the Hub; the watcher that polls the `Deployer` and joins deployments to releases; the local-project → Panacea-project binding | No — they hold a `deploy.Deployer` |
| Surface | `control-plane/internal/api/deploy*.go`; `desktop/src/deploy/…` | control-plane routes; the Deploy lens | No |

**The swap (OD-13).** When Panacea is lifted into this repo, a second implementation of
`deploy.Deployer` — native, in-process, or over the lifted service's internal API — replaces
`panacea.Client` in `cmd/api/main.go`'s wiring. The domain types, the orchestration, the routes and
the lens do not change. The conformance suite `deploy/deploytest.Conformance(t, d Deployer)` is the
proof: it runs today against the fake and against `panacea.Client` over a recorded `httptest`
server, and it is what the native implementation must pass. §9 names this seam S7-N1.

**Data flow for a read.** The watcher polls the `Deployer` for every bound project on its own
timer (default 15 s; 3 s while a run the user shipped is in flight). It holds the newest snapshot in
memory and bumps a per-process **deploy cursor** when anything in it changes. The control plane sets
`Panacea-Deploy-Cursor: <n>` on its `GET /v1/sessions` response — the one request `poller.ts` makes
each tick — exactly as the Hub contract §9.3 does for `Panacea-Hub-Cursor`. When the desktop sees
the cursor change it makes **one** `GET /v1/projects/{id}/deploy`. So the steady state adds **zero**
requests per tick, the one-clock rule holds (two `setInterval` sites, `poller.ts:84` and
`hooks/heartbeat.ts:34`, and phase 7 adds none), and Panacea is polled by one process at a rate the
control plane chooses, not by the renderer's tick.

**Data flow for Ship** (Hub contract §7.9, graft G1 of ADR-H1):

1. Desktop `POST /v1/projects/{id}/deploy/ship {branchId, commit, designKey?}`.
2. Control plane checks preconditions it can check locally (connected; bound; the commit is the
   branch tip and phase 3's `remote` block says `ahead == 0 && behind == 0`; Hub signed in).
3. `hubclient.Releases.Reserve` → **the Hub fixes the pin**. Failure → Ship does not happen.
4. `Deployer.Ship` → Panacea `POST /api/v1/projects/{name}/deploy {branch, commit}` → `202` with
   `pipelineRun`.
5. `hubclient.Releases.Confirm {pipelineRun}` — or, on Panacea's refusal, `Abandon {reason,
   message}` carrying Panacea's sentence verbatim.
6. The watcher, the first time a `Deployment` with `ID == pipelineRun` appears, writes the
   deployment join (`JoinDeployment`). **`Deployment.ID` is the PipelineRun name**
   (`clients/kubernetes.go:5743`, `ID: pr.GetName()`, inside `ListDeployments` at `:5596`), so
   the join key is known before the row exists, and nothing is inferred.

Panacea paths below are written relative to `~/Projects/Panacea/services/panacea-api/` and were
read at Panacea commit `b33d15acb`. **Line numbers have moved since the survey** (`Rollback` was
`projects.go:1629`, is `:1547`; `TriggerDeploy` was `:3024`, is `:2942`); every Panacea line cited
in this plan was re-read on 2026-09-23.

---

## 3. The decisions this plan rests on

### 3.1 Owner decisions (`planning/2026-09-23-owner-decisions.md`)

| Id | What phase 7 does because of it |
|---|---|
| **OD-1** | Built on the briefs' recommendations; UI branch stays unmerged until the owner has looked |
| **OD-3** | Releases and their pins live in the **Hub**, not the state directory. This **answers brief decision `D1`** (per person vs per project) with its option *b* and **without** the Panacea metadata ask: a teammate's Ship reserved its pin in the same Hub my cockpit reads, so their release shows its brief on my screen |
| **OD-4** | The binding (§3.3, 7P-6) matches a Panacea project's `repoUrl` against the local remote through phase 3's `githost.ParseRemote`, which recognises GitHub and Azure DevOps. Nothing in phase 7 names a host |
| **OD-6** | Identity is the Hub's. Phase 7 never runs a sign-in; it takes `hubclient.Releases` (which carries the Entra token) and `credstore.Secrets` (which holds the `pnc_` token) from the Hub's tasks C1/C2 |
| **OD-12** | Every owner-observed step — the live Ship against dev Panacea, the lens's look — is batched to the end and stays **unverified** until then |
| **OD-13** | Panacea is called as a client over its existing API; **no Panacea source is copied**; the deploy side is the `deploy.Deployer` interface so the lift is a swap (§2, S7-N1) |

### 3.2 Brief, contract and ruling decisions adopted

| Id | Source | Adopted as |
|---|---|---|
| `D1` | `scope-and-risk-phase-7-deploy.md` §0 | answered by OD-3 (above) |
| `D2` | same | **(a) always** — collisions shown after the fact (task 08); **URL prefix in scope**, owner-only (task 11); **(c)** — ask Panacea to hash-suffix truncated names — is an outward request, on the owner's list (§10 Q-P7-3) |
| `D3` | same | **(b)**: phase 3 stamps `Panacea-Session` on every agent commit (phase 3 plan `:23`, `3-D6`, seam `S3-P7.1`); the manifest lists sessions from the trailers in the shipped range (task 18) |
| `B1` | same §0.1 | two access controls, drawn as the API offers them — **corrected**: project access is now shared with a **person** (`grantee` username or email, `clients/keycloak.go:180-190`), not with a group; legacy group bindings still list, with an empty `grantee` (`models/models.go:2004-2015`). The brief read an older API |
| `B2` | same | Ship sends **no** scan-mode override; the lens shows each scan's `mode` |
| `B3` | same | Ship, rollback, rebuild and access writes are **held for 5 s with Undo** before any request leaves the desktop (7P-15) |
| `B4` | same | times are absolute (`Deployment.createdAt`, the Hub's `reservedAt`) |
| Q1 | provenance contract §7.2; ADR-H1 Q1 | the Hub pins the **latest complete** revision at reserve; the manifest shows `headRevisionAtShip` when it differs |
| Q2 | provenance contract §7.2 | releases shipped outside the cockpit read *"no brief is recorded for this release"*. With auto-deploy on, every push is such a release, and the auto-deploy control says so |
| Q6 | provenance contract §6.6 | answered by OD-6 + the Hub's local caller key (Hub contract §9.4, task C3): the `pnc_` token sits behind a control-plane surface that another OS user or a sandbox cannot drive |
| **R4** | `orchestrator-rulings.md` | Panacea unreachable is `503 upstream_unavailable`, `details.upstream = "panacea"`, classified as the fourth `ProblemKind`, `upstream-down`. Phase 7 adds the kind **unless phase 5 has already** (task 13) |
| **R5** | same | the release key is the Hub's tagged union. **The premise is now verified** (§3.3, 7P-4) |
| **R6** | same | Deploy contributes **one selector** of release pin keys to phase 4's loader (task 13); it adds no interval and no per-row fetch |
| **R7** | same | the Panacea metadata ask is **drafted** in task 19; sending it is the owner's (§10 Q-P7-2) |
| **G1** | ADR-H1 §4.2; Hub contract §7.9 | Ship is **reserve → deploy → confirm or abandon**; the Hub, not phase 7, selects the pinned revision |
| relay | Hub contract §4.3 | a Hub 4xx reaches the desktop unchanged; Hub down is `upstream_unavailable` with `upstream = "hub"` |

### 3.3 This plan's own decisions — each with its evidence and its cost if wrong

**7P-1 — The control plane authenticates to Panacea with the user's own `pnc_` token, held in
Windows Credential Manager. Not with an Entra token.**

The task asked for this to be decided and justified. The Entra option fails on four facts read
from Panacea's source, any one of which is enough:

1. **`panacea-api` does not accept a user JWT as a bearer.** `middleware.AuthWithOptions`
   (`middleware/auth.go:135-262`) reads an identity JWT only as `X-Backstage-Identity`, and only
   after `Authorization: Bearer` has matched `BACKEND_SECRET` in constant time (`:160-173`). A bare
   `Bearer <jwt>` that is neither the secret nor a `panacea_` legacy key falls through to
   `401 "unauthorized: missing or invalid authentication"` (`:245-251`). The desktop must never
   hold `BACKEND_SECRET`: it is one shared service secret, and the header path under it is the one
   Panacea's own code calls *"an UNVERIFIED claim about the END USER"*.
2. **The verifier is single-issuer, single-audience.** `backstageVerifierFromEnv`
   (`middleware/jwks.go:149-151`) checks one `BACKSTAGE_JWT_ISSUER` and one
   `BACKSTAGE_JWT_AUDIENCE`. Our desktop's Entra access token has `aud` = the **Hub API's** client
   id (Hub contract §6.3).
3. **Forwarding that token would break ADR-H1 §5.8**: *"Hub, broker, Azure DevOps and Panacea tokens
   are four audiences, and none is ever forwarded to another service."*
4. **Getting a Panacea-audience Entra token** would need a Panacea app registration exposing a
   delegated scope, our desktop pre-authorised on it, and `panacea-api` taught to accept a raw
   bearer JWT on its REST chain. All three are changes to Panacea, which `OD-13` keeps separate and
   which is being updated by another team right now. `VerifyBearerToken` (`auth.go:475`) exists,
   but it serves the MCP and conduit surfaces (`routes/mcp_wiring.go:117`, `routes.go:3296`), not
   the REST chain phase 7 needs.

The `pnc_` path is first-class: `ApiKeyWithGroups` (`middleware/api_key.go:127`) runs **before**
`Auth` in every chain phase 7 uses (`authed`, `authedProject`, `authedProjectDeveloper`,
`authedProjectOwner`, `routes/routes.go:3226-3330`); it is per user, revocable, and intersected with
the owner's live group membership when `LiveGroupResolver` is wired (survey §5.4). It is what
`services/panacea-mcp` uses (`src/index.ts:19`, `:57`). The user is always themselves; nothing is
pooled.

- **Cost:** one paste at onboarding. The app cannot mint the token: `POST /api/v1/me/api-keys` is
  `authedUnscoped` + `RequirePortalGrant` (`routes.go:3050`), so it needs a browser session.
- **Where it lives:** `credstore.Secrets` target `PanaceaApp/panacea/<api host>` — the row the Hub
  contract §9.1 already reserves for it. Written through a **write-only** control-plane route
  (task 05); no response ever carries it; it is never logged; it never enters a sandbox.
- **The swap:** `deploy.Deployer` hides the credential kind entirely. The native deployer after the
  lift authenticates with the Hub's Entra identity; the `pnc_` row is then deleted by a migration
  step named in S7-N1.
- **Cost if wrong:** if Panacea later accepts Entra bearer tokens for our audience, `panacea.Client`
  gains a second `Authorizer` and the connect screen loses its paste step. Nothing above the
  connector changes.

**7P-2 — One domain interface, `deploy.Deployer`, and a conformance suite that every
implementation passes.** Complete source in task 01. It speaks in our types (`deploy.Environment`,
`deploy.Deployment` with `LiveReady *int`, `deploy.RunProgress`, `deploy.ScanReport`,
`deploy.AccessPosture`), never Panacea's. *Cost if wrong:* a lift that needs a method the interface
lacks adds one; the suite shows what else must hold.

**7P-3 — "Rebuild this release", not Panacea's `/redeploy`.** Panacea's `Redeploy` builds
**the branch head**: `TriggerDeployOptions{Branch: branch, Commit: branch, …}`
(`handlers/projects.go:1886-1889`). The provenance contract (§3.4) says a redeploy inherits the pin
of the deployment it re-runs, which is only true if it re-runs that deployment's **commit**. So
phase 7's Rebuild calls `POST …/deploy {branch: source.Branch, commit: source.Commit}` and reserves
a `run` **with** `sourceDeploymentId`, which inherits the source's pin (Hub contract §7.9, pin
table row 2). An unchanged commit reuses the built image — `manualImageTag` is deterministic for a
real SHA (`clients/pipeline.go:205`ff, the WP-31 comment). A source with no recorded `Commit`
cannot be rebuilt exactly; the control is disabled with that reason and *Ship the branch* is
offered. **The user-visible label is "Rebuild", not "Redeploy"** — assertion A-P7-1 (§10), which
the owner may overrule. Panacea's `reuseImage` config-only restart is not offered in v1.
*Cost if wrong:* one extra route mapping.

**7P-4 — Rollback creates no PipelineRun; its confirm carries no deployment id.** Ruling R5's
premise, **verified**: `K8sClient.RollbackDeployment` (`clients/kubernetes.go:5885-6038`) either
flips the stable Service alias to the still-materialised immutable Deployment (fast path) or
patches the image of the legacy `app` Deployment (fallback). Neither creates a PipelineRun, and
the object it returns has `ID: deployID` — **the id of the deployment rolled back to**, not a new
one (`:5982`, `:6030`). No handler writes a deployment row for a rollback: the only `Trigger`
values set anywhere are `git-push`, `redeploy`, `prod-branch-change` and `manual`
(`handlers/code_push.go:189`, `handlers/projects.go:1888`, `:2631`, `:2972`, `:3056`, `:3116`;
`clients/deploy_poller.go:186`). So:

- phase 7 reserves `{kind: "rollback", panaceaProject, environment, sourceDeploymentId}`;
- confirms with **`{}`**. Sending Panacea's returned id would try to join the **source's** id,
  which the source release already holds — Hub contract §7.9 answers that with `409 conflict`;
- and a rollback **appears in no Panacea list**. `deploys/latest` still reports the newer build
  after a rollback. The lens therefore reads "what is serving" as the later of the environment's
  newest `Deployment.createdAt` and its newest **confirmed rollback release**'s `confirmedAt` from
  the Hub (task 08). A rollback made in the Panacea portal is invisible to us, and the lens's help
  text says so.

Two more facts from the same reading, both drawn in task 10: `Rollback` finds its target only in
`K8s.ListDeployments` — the **live** PipelineRuns (`projects.go:1555-1572`) — so a rollback to a
deployment whose run Tekton has pruned (~7 days) is `404 "deployment not found: …"` even though
`RollbackDeployment` could recover it; and the warmth hint (`rollbackWarmth`,
`coldStartSeconds`) arrives **only on the response** (`models/models.go:695-708`), so the lens shows
it after the act, not before.

**7P-5 — Panacea is polled by the control plane, and the desktop learns of changes through a
cursor header.** §2. The watcher's rates are configuration (`PANACEA_APP_DEPLOY_POLL_INTERVAL`,
`…_ACTIVE_POLL_INTERVAL`). `GET …/deployments` is **not** polled each cycle: it is unpaginated and
runs three enrichment passes including *"a dozen PipelineRun reads"* (`projects.go:1227-1253`), so
the watcher reads it only when `deploys/latest` or `environments` changed, and at most once a
minute otherwise. *Cost if wrong:* rates are variables.

**7P-6 — A local project is bound to its Panacea project automatically, by repository.** The
watcher lists the Panacea projects the token can see (`GET /api/v1/projects`, `ListResponse` of
`Project` with `repoUrl`, `models/models.go:567`) and matches `githost.ParseRemote(repoUrl).RepoKey()`
against the local project's `origin`. One match binds silently. None says *"No Panacea project
deploys this repository"*. Several — Panacea allows projects that share a repo slug
(`models/models.go:2388-2389`, the `ProjectName`/`ProjectNamespace` comments) — ask the user once;
the choice is stored **locally**, in `statepath.Dir("deploy")/bindings.json`, written with the
shared `writeAtomic` (ruling R2). A shared binding in the Hub is Hub ask HA7-3 (§7), not a blocker.
*Cost if wrong:* a teammate with an ambiguous repo picks once.

**7P-7 — Ship ships the branch tip, and only when the remote holds exactly that commit.**
`TriggerPipelineRun` clones `project.RepoURL` (`clients/pipeline.go:174`), so Panacea can build only
what has been pushed. Phase 3's convergence read model reports `remote.ahead` and `remote.behind`
against the upstream (phase 3 task-08 `remote.State`). Ship is enabled when `upstream != ""`,
`ahead == 0` and `behind == 0`, and sends `commit = tip`. Otherwise Ship is disabled with the reason
(*"2 commits are not pushed, so they would not ship"* with phase 3's Push beside it; *"the remote
has 3 commits this clone has not fetched"*). This avoids asking phase 3 for the upstream's SHA and
never ships a commit the user cannot see. `commit` is 40 lowercase hex, which Panacea's
`commitSHARe` (`clients/pipeline.go:33`, `^[0-9a-f]{7,40}$`) and the Hub (§7.9) both accept.

**7P-8 — Environments are Panacea's names, joined by name, and a name that joins nothing is shown,
not dropped.** `GET …/environments` (`projects.go:1997-2153`) names the base environment
`"production"`, while a deployment's `Environment` is the PipelineRun label `panacea.io/environment`
(`clients/kubernetes.go:5681`), whose value for production **was not verified** — other handlers use
`"prod"` (`canonicalStageEnv`, `projects.go:4124-4126`). The view joins by exact name; a deployment
whose environment matches no row appears in its own group, titled with its raw name, and task 19's
live check settles the spelling. **The environment for a first Ship** of a never-deployed branch is
not knowable without re-implementing `sanitizeBranchEnv` (`projects.go:3814-3826`), which the survey
rightly forbids (a fourth copy to drift). Hub ask **HA7-1** makes `environment` optional at reserve
and settable once at the deployment join. The plan builds on it.

**7P-9 — Owner-only controls are detected by asking the owner-only route, never by parsing group
names.** `GET …/branches/{branch}/auto-deploy` is `authedProjectOwner` (`routes.go:1308`): a
non-owner gets `403` from the middleware before the handler runs, an owner gets `200` or the
handler's `409 "branch has no environment yet"` (`projects.go:4132-4134`). The watcher records that
status as `ownerControls: "yes" | "no" | "unknown"`. Parsing `project-<name>-owner` out of `/me`'s
groups would be a second reader of a namespace whose own file says it has exactly one
(`models/project_groups.go:5`). Developer rights are **not** probed — no developer-only GET exists
in the routes phase 7 uses — so a viewer's Ship is refused by Panacea and rendered verbatim, after
the reservation is abandoned.

**7P-10 — Corporate TLS through the operating system's trust store; no "insecure" switch.**
`panacea-mcp` needed `PANACEA_TLS_INSECURE=1` (`src/index.ts:69-71`) because Node carries its own
CA list. Go's `crypto/x509` on Windows verifies through the Windows certificate store, where KPMG's
interception root is installed — **from knowledge, not tested here**; task 02's acceptance includes
one live `GET /api/v1/version` from Go on this laptop. `PANACEA_APP_PANACEA_CA_FILE` adds a PEM for
non-Windows builds. There is no skip-verify flag at all.

**7P-11 — The Panacea address is an origin, validated before it is accepted.** Measured by the
orchestrator: `https://dev.panacea.kpmg/api/v1/version` answers `200 {"version":"v1.163.0"}`; the
`api.dev…` host is behind SSO (`orchestrator-rulings.md`, "Settled by measurement"). Every route in
`routes.go` already begins `/api/v1/`, so the configured value is the **origin**
(`https://dev.panacea.kpmg`), and a value with a path is refused with a sentence naming the fix.
`PANACEA_APP_PANACEA_URL` sets it; otherwise the connect form asks, and the control plane accepts
it only after an unauthenticated `GET /api/v1/version` returns JSON with a `version` string.

**7P-12 — Panacea's refusals become our codes, carrying Panacea's sentence.** Every Panacea error
body is `{"error":"<sentence>"}` — `writeError` → `models.APIResponse{Error}`
(`handlers/helpers.go:41-43`), `writeAPIKeyError` (`middleware/api_key.go:396-400`), and
`http.Error` with a JSON literal (`auth.go:250`, `rbac.go:398-400`; that last form arrives as
`text/plain`, so the decoder reads the body regardless of `Content-Type`). The mapping is task 02's
table: transport failure, timeout, `5xx` and `429` → `503 upstream_unavailable`
(`upstream = "panacea"`); `401` → `409 deploy_token_rejected`; `403` → `403 deploy_forbidden`; `404`
→ `404 deploy_not_found`; `409` → `409 deploy_conflict`; `400`/`422` → `422 deploy_rejected`. The
`message` is Panacea's sentence, trimmed to 500 bytes; **a `5xx` sentence is never shown** (Panacea
appends `err.Error()` to them, e.g. `projects.go:1529`), only logged. *"Waiting for team approval"*
is read from `GET /api/v1/me`'s `approved` field (`handlers/me.go:48-59`), **never** by matching the
403 sentence.

**7P-13 — Nothing Panacea says is persisted.** Deploy data is server truth held in the watcher's
memory; the desktop slice is not persisted (provenance contract A5, widened by ADR-H1 §5.4). The
only thing phase 7 writes to disk is the ambiguous-binding choice (7P-6), which is configuration,
not Hub or Panacea data. **No persist version is claimed** (ruling R3).

**7P-14 — SSE is relayed for one thing at a time.** The pipeline-log stream
(`routes.go:2283`) and the readiness stream (`:2286`) are relayed by the control plane only for the
one run or environment the user has drilled into, and the desktop reads them with `fetch` and a
stream reader, because `EventSource` cannot send the Hub's local caller key header (Hub contract
§9.4).

**7P-15 — Undo is "not sent yet".** Panacea cannot un-start a pipeline or un-flip a Service, so
the only honest undo for Ship, rollback, rebuild and access writes is to hold the request in the
desktop for 5 s after the press, with the consequence printed beside the button **before** it is
pressed (B3). Undo cancels before any request leaves. After 5 s the request is sent and the row
shows what happened. Cancel of a running pipeline is itself the undo of Ship after that point.

---

## 4. The Panacea surface phase 7 uses — every route re-read in `routes/routes.go`

Guards are the chain wrappers at `routes.go:3226-3333`. **Envelopes differ per route**, and the
connector decodes each exactly: `bare` = the object itself; `list` = `models.ListResponse
{items,totalCount}` (`models/models.go:2153`); `data` = `models.APIResponse{data,error,message}`
(`:2147-2151`).

| # | Method, path (`/api/v1/projects/{name}` = `P`) | `routes.go` | Guard | Answer | Used by task |
|---|---|---|---|---|---|
| 1 | `GET /api/v1/version` | 630 | none | bare `{"version"}` | 02 |
| 2 | `GET /api/v1/me` | 3038 | authed | bare `{username,email,groups,isAdmin,role,approved}` (`handlers/me.go:48-59`) | 03, 05 |
| 3 | `GET /api/v1/projects` | 1018 | authed | list of `Project` | 03, 06 |
| 4 | `GET P/environments` | 1126 | member | list of rows `{name,branch,displayName,primary,subdomain,status,lastDeploy,replicas?,previewData?}` (`projects.go:2116-2152`) | 03, 07 |
| 5 | `GET P/deploys/latest` | 1025 | member | bare `Deployment`, newest **across all environments**; `404` when none (`projects.go:1521-1546`) | 03, 07 |
| 6 | `GET P/deployments` | 1024 | member | list of `Deployment`, unpaginated, enriched (`projects.go:1227-1253`) | 03, 07 |
| 7 | `GET P/pipelines/{run}` | 1290 | member | data `PipelineRunStatus` (`projects.go:3576-3612`) | 03, 07 |
| 8 | `GET P/security-status?deployId=` | 1043 | member | data `SecurityStatus` (`models/security.go:205`; `handlers/security_status.go:204-254`); `deployId` is a PipelineRun name (`:211-214`) | 03, 12 |
| 9 | `GET P/environments/{env}/pods` · `/events` · `/diagnose` | 1148, 1150, 1151 | member | passed through, shape not interpreted | 12 |
| 10 | `GET P/pipelines/{run}/logs/stream` · `P/environments/{env}/readiness/stream` | 2283, 2286 | member | SSE, relayed byte for byte | 12 |
| 11 | `POST P/deploy` | 1286 | developer + `RequireSectionWritable("service")` | `202` data `{project,status,pipelineRun}` (`projects.go:2997-3005`) | 04, 09 |
| 12 | `POST P/pipelines/{run}/cancel` | 1291 | developer | `200` data `{status: "cancelled" \| "already_completed", message}` (`projects.go:3668-3687`) | 04, 10 |
| 13 | `POST P/deployments/{id}/rollback` | 1026 | developer | `200` data `Deployment` with `rollbackWarmth`, `coldStartSeconds`; `404`, `400` (no image), `409` (image pruned) (`projects.go:1547-1640`) | 04, 10 |
| 14 | `GET` / `PUT P/branches/{branch}/auto-deploy` | 1308, 1309 | **owner** | data `{branch,enabled,projectDefault,effective,isProductionBranch}`; `409` when the branch has no environment (`projects.go:4114-4147`); PUT body `{"enabled": bool}` (`:4160-4167`) | 04, 11 |
| 15 | `GET` / `PUT P/branches/{branch}/access` | 1430, 1431 | member / developer | data `{branch,env,accessMode,ipWhitelist,passwordSet,bypassTokenCount}` (`handlers/project_security.go:668-677`); PUT `{accessMode, password?, bypassTokens?, rotateBypassTokens?}` (`:681-691`) | 04, 11 |
| 16 | `GET` / `POST P/access` · `DELETE P/access/{group}` | 1407-1409 | member / **owner** / **owner** | list of `AccessBinding{group,role,grantee?}` (`models/models.go:2004-2015`); POST `{group,role}` → `201` (`handlers/access.go:17-61`) | 04, 11 |
| 17 | `PUT` / `DELETE P/branches/{branch}/url-prefix` | 1313, 1314 | **owner** | PUT `{"prefix"}` → data `{branch,prefix,host}`; `409` taken (`projects.go:4263-4373`) | 04, 11 |

**Not used, deliberately:** `P/environments/{env}/redeploy` (7P-3), `P/promote`, the branch
lifecycle verbs (archive, pause, delete, set-primary), `POST /api/v1/me/api-keys` (a token cannot
mint a token), every admin route, and the ten scan-mode overrides on `TriggerDeployOptions`
(`models/models.go:2342-2383`, closing before `PipelineRunStatus` at `:2384`; ten, as the scope brief corrected the survey's eleven).

**Two facts the lens must respect**, both from the handlers above:
- **`deploys/latest` is project-wide, not per environment.** The per-environment header comes from
  `environments` (row 4) plus the newest `deployments` row per environment, which alone carries
  `liveReadyReplicas` (`models/models.go:709-723`, *"Only the NEWEST deploy per environment carries
  it"*).
- **`POST P/access` now shares a project with a person.** `group` carries a username or email that
  `AddProjectAccess` resolves to a Keycloak user (`clients/keycloak.go:180-192`) and adds to
  `project-<name>-<role>`. The role vocabulary on the wire is the group suffix: `viewer`, `editor`
  (developer) and `owner` (`models/project_groups.go:9-12`). The lens labels `editor` *Developer*.
  **Unverified:** whether `developer` is accepted as a synonym on POST; task 04 sends `editor`.

---

## 5. Global constraints

Everything in phase 2's Global Constraints (`2026-09-18-phase-2-backend-and-cockpit.md:42-60`)
holds. In particular:

- **`go test ./...` is invalid at the workspace root.** Run
  `go test ./control-plane/... -race -count=1` (and `./sandboxd/...` when a task touches it).
- `go 1.23` stays in `go.work` and `control-plane/go.mod`. **Phase 7 adds no module requirement**;
  check `git diff control-plane/go.mod` is empty before every commit.
- Desktop: `cd desktop && npx vitest run` and `npx tsc --noEmit`, both green.
- Every shell command through `~/.claude/tools/claudezsh/claudezsh`. Commit by pathspec:
  `git commit -F <file> -- <paths>`; a new file is `git add <path>` first. Never
  `add -A/-u/.`, stash, reset, checkout, switch, rebase, merge, amend or push.
- Every commit message ends with exactly:
  `Co-Authored-By: Claude Opus 5.5 (1M context) <noreply@anthropic.com>`

**What never changes, in every task of this phase:**

1. **Agent CLIs are never modified, wrapped or parsed; nothing derives meaning from PTY bytes.**
   Phase 7 reads no transcript, so OD-8's adapter rule does not arise. Session attribution (task 18)
   reads **git trailers**, which phase 3's hook writes into commits.
2. **Never pool across users.** The `pnc_` token is the signed-in user's own; one per machine user;
   never sent to the Hub, a sandbox, a log line or the renderer.
3. **No secret in source.** Configuration is `PANACEA_APP_*` (§6, task 05). No default Panacea
   address is compiled in.
4. **Loopback only.** Phase 7 adds routes to the existing listener and opens no listener of its own.
5. **No `GET` changes state** (phase 2 contract §10a). Every phase 7 `GET` is added to
   `catalogReadOnlyRoutes` in `control-plane/internal/api/routes_audit_test.go` with its reason. The
   SSE relays are reads.
6. **No Panacea source copied** (OD-13). DTOs are re-declared with only the fields used, each with
   the Panacea file and line it mirrors.
7. **One clock in the desktop.** No new `setInterval`, no per-row `useEffect` fetch (P8), no spinner
   (DESIGN-SYSTEM.md "Designing for many live processes").
8. **Provenance prohibitions P1–P13** (provenance contract §5) — in particular P1 (never render a
   release from the head), P2 (never pin by observation), P4 (never copy brief text into a store),
   P5 (never parse a key outside `provenance`), P7 (no second-call link) and P9 (never hide an
   absent pin).
9. **Build status, serving status and scans are three facts**, always drawn separately. A `nil`
   ready count says nothing; `0` says down.



