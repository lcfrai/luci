# Phase 7 — the Deploy connector: implementation plan

> **For agentic workers:** execute task by task with `superpowers:subagent-driven-development` or
> `superpowers:executing-plans`. Every task's part file is in
> `docs/superpowers/plans/2026-09-23-phase-7-deploy-parts/task-NN.md`. Steps use `- [ ]` checkboxes.
> Every task is test-first.

**Status:** written 2026-09-23 on branch `feat/phase-7-deploy` (worktree
`.claude/worktrees/phase-7`, based on `3303fd6`); parts 17–20 finished, and the file map (§7),
task table (§8) and seams (§10.2) re-checked against every part, on 2026-09-24. Owner-reviewable
under `OD-1`; adversarially reviewed before code. **Nothing in this plan was compiled or run.** Every citation below was opened
in this pass unless it is marked *unverified*; §12 lists what was not verified.

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
the lens do not change. The conformance suite `deploy/deploytest.Conformance(t, h Harness)` (a
`Harness` hands it a `Deployer` and a `Seeder`, task 01) is the
proof: it runs today against the fake and against `panacea.Client` over a recorded `httptest`
server, and it is what the native implementation must pass. §10.2 names this seam S7-N1.

**Data flow for a read.** The watcher polls the `Deployer` for every bound project on its own
timer (default 15 s; 3 s while a run the user shipped is in flight). It holds the newest snapshot in
memory and bumps a per-process **deploy cursor** when anything in it changes. The control plane sets
`Panacea-Deploy-Cursor: <n>` on its `GET /v1/sessions` response — the one request `poller.ts` makes
each tick — exactly as the Hub contract §9.3 does for `Panacea-Hub-Cursor`. When the desktop sees
the cursor change it makes **one** `GET /v1/projects/{id}/deploy`. So the steady state adds **zero**
requests per tick, the one-clock rule holds (two `setInterval` sites, `poller.ts:90` and
`hooks/heartbeat.ts:34`, and phase 7 adds none), and Panacea is polled by one process at a rate the
control plane chooses, not by the renderer's tick.

**Data flow for Ship** (Hub contract §7.9, graft G1 of ADR-H1):

1. Desktop `POST /v1/projects/{id}/deploy/ship {branchId, commit, designKey?, noCache?}`, with an
   `Idempotency-Key` minted when its 5 s hold starts (task 16).
2. Control plane checks preconditions it can check locally (connected; bound; the commit is the
   branch tip and phase 3's `remote` block says `ahead == 0 && behind == 0`; Hub signed in).
3. `hubclient.Releases.Reserve` → **the Hub fixes the pin**. Failure → Ship does not happen.
4. `Deployer.Ship` → Panacea `POST /api/v1/projects/{name}/deploy {branch, commit}` (plus
   `noCache` only when set; never a scan override) → `202` with `pipelineRun`.
5. `hubclient.Releases.Confirm {pipelineRun}` — or, on **any** Panacea failure, `Abandon {reason,
   message}`: Panacea's sentence verbatim for a refusal, a fixed sentence for every other error
   (task 09 step 6, which has no branch that leaves a reservation to expire; plan re-review b05 R-2).
6. The watcher, the first time a `Deployment` with `ID == pipelineRun` appears, writes the
   deployment join (`JoinDeployment`). **`Deployment.ID` is the PipelineRun name**
   (`clients/kubernetes.go:5743`, `ID: pr.GetName()`, inside `ListDeployments` at `:5596`), so
   the join key is known before the row exists, and nothing is inferred.

Panacea paths below are written relative to `~/Projects/Panacea/services/panacea-api/`, and their
line numbers are those of Panacea commit `406d0da95`. **Line numbers have moved since the survey**,
which read `b33d15acb`: `406d0da95` removed 82 lines from `projects.go` after `:1418` (`Rollback`
was `projects.go:1629`, is `:1547`; `TriggerDeploy` was `:3024`, is `:2942`). Every Panacea line
cited in this plan was re-read on 2026-09-23; the lines parts 17–20 cite, and a sample of the rest,
were re-read at `b02424691` on 2026-09-24. Between `406d0da95` and `b02424691` the only
`panacea-api` source that changed is `clients/kubernetes.go`, from `:6468` down (plus a new test
file), and every `kubernetes.go` line this plan cites is above `:6100`, so every cited line is where
it was.

---

## 3. The decisions this plan rests on

### 3.1 Owner decisions (`planning/2026-09-23-owner-decisions.md`)

| Id | What phase 7 does because of it |
|---|---|
| **OD-1** | Built on the briefs' recommendations; UI branch stays unmerged until the owner has looked |
| **OD-3** | Releases and their pins live in the **Hub**, not the state directory. This **answers brief decision `D1`** (per person vs per project) with its option *b* and **without** the Panacea metadata ask: a teammate's Ship reserved its pin in the same Hub my cockpit reads, so their release shows its brief on my screen |
| **OD-4** | The binding (§3.3, 7P-6) matches a Panacea project's `repoUrl` against the local remote through phase 3's `githost.ParseRemote`, which recognises GitHub and Azure DevOps. Nothing in phase 7 parses a host or copies a key pattern: task 06 asks the Hub's own `hubapi.HostOfRepoKey` whether a missed key is in a form the Hub accepts, and that only chooses a sentence. **Azure DevOps Ship rests on HA7-6** (§9): phase 3's `RepoKey()` and the Hub must spell Azure DevOps alike. The plans agree (the Hub's correction A1 takes phase 3's form); the Hub contract's §7.4 text and phase 3's H-3 row still show the old form (review I6, plan re-review b05 R-1) |
| **OD-6** | Identity is the Hub's. Phase 7 never runs a sign-in; it takes `hubclient.Releases` (which carries the Entra token) and `credstore.Secrets` (which holds the `pnc_` token) from the Hub's tasks C1/C2 |
| **OD-12** | Every owner-observed step — the live Ship against dev Panacea, the lens's look — is batched to the end and stays **unverified** until then |
| **OD-13** | Panacea is called as a client over its existing API; **no Panacea source is copied**; the deploy side is the `deploy.Deployer` interface so the lift is a swap (§2, S7-N1) |

### 3.2 Brief, contract and ruling decisions adopted

| Id | Source | Adopted as |
|---|---|---|
| `D1` | `scope-and-risk-phase-7-deploy.md` §0 | answered by OD-3 (above) |
| `D2` | same | **(a) always** — collisions shown after the fact (task 08); **URL prefix in scope**, owner-only (task 11); **(c)** — ask Panacea to hash-suffix truncated names — is an outward request, on the owner's list (§10 Q-P7-3) |
| `D3` | same | **(b)**: phase 3 stamps `Panacea-Session` on every agent commit (phase 3 plan `:23`, `3-D6`, seam `S3-P7.1`); the manifest lists sessions from the trailers in the shipped range (task 19) |
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
| **R7** | same | the Panacea metadata ask is **drafted** in task 20; sending it is the owner’s (§11 Q-P7-2) |
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
  step named in S7-N1 (§10.2).
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
phase 7's Rebuild calls `POST …/deploy {branch: <the source's Hub release branchRef>, commit:
source.Commit}` and reserves a `run` **with** `sourceDeploymentId`, which inherits the source's pin
(Hub contract §7.9, pin table row 2). The branch is never `source.Branch`: that is Panacea's run
label — `sanitizeLabelValue(branch)` from the API (`clients/pipeline.go:609`) or `sanitizeLabel`
from the webhook-receiver (`webhook-receiver/pipeline/trigger.go:1270`) — so `feat/x` reads `feat-x`
(§12). A
source with no Hub `branchRef` is refused `no_branch` (task 10; b02 round 2, R8). An unchanged
commit reuses the built image — `manualImageTag` is deterministic for a
real SHA (`clients/pipeline.go:205`ff, the WP-31 comment). A source with no recorded `Commit`
cannot be rebuilt exactly; the control is disabled with that reason and *Ship the branch* is
offered. **The user-visible label is "Rebuild", not "Redeploy"** — assertion A-P7-1 (§11, Q-P7-1), which
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
  after a rollback. The lens therefore reads "what is serving" as the later of two facts (task 08
  rule 6, review I3): the environment's newest **succeeded** deployment by **completion**
  (`createdAt` + Panacea's `duration`, `models/models.go:682`; computed at
  `clients/kubernetes.go:5683-5689`), and its newest **confirmed rollback release**'s
  `confirmedAt` from the Hub. `createdAt` alone is PipelineRun creation (`kubernetes.go:5680`,
  `:5754`), not go-live, and a failed or running build is never current. A rollback made in the
  Panacea portal is invisible to us, and the lens's help text says so.

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
against the local project's `origin`. One match binds silently. None says *"No Panacea project your
key can see deploys this repository. …"* (the full sentence is task 06's table, t06:154). Several — Panacea allows projects that share a repo slug
(`models/models.go:2388-2389`, the `ProjectName`/`ProjectNamespace` comments) — ask the user once;
the choice is stored **locally**, in `statepath.Dir("deploy")/bindings.json`, written with the
shared `writeAtomic` (ruling R2). A shared binding in the Hub is Hub ask HA7-3 (§9), not a blocker.
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
(`clients/kubernetes.go:5681`). On the deploy path its value for production is `"prod"`:
`TriggerPipelineRun` computes `env := environmentForBranch(branch, prodBranch)`
(`clients/pipeline.go:237`, the function at `:2195-2207`) and writes it unchanged as the label at
`:608` (read from source in review b02). A push writes `"prod"` too (`webhook-receiver/models/
event.go:116-129`, the label at `pipeline/trigger.go:1271`). The one other writer, the
security-advisory run's `TargetEnv` label (`pipeline.go:842`), never becomes a deployment:
`ListDeployments` drops every advisory run before it builds a row (`clients/kubernetes.go:5633-5642`,
`isAdvisoryRun` at `:140-142`). So production's label is `"prod"`, settled from source (review M5,
§12 *Settled from source*). Other
handlers also use `"prod"` (`canonicalStageEnv`, `projects.go:4124-4126`). Panacea's own `tenantNamespace`
(`clients/kubernetes.go:4210-4217`) treats `prod` and `production` as **the same** environment
(both map to `tenant-<project>`), so the view joins by exact name **plus that one equivalence**,
cited to that function and pinned by a test that quotes it. Any other deployment whose environment
matches no row appears in its own group, titled with its raw name — never merged, never dropped —
and task 20's live check records the real spelling. **A legacy run with no environment label**
reads back as `""` (`kubernetes.go:5681`; every current writer sets one, task 12). It is joined the
way Panacea's own environments list attributes it: by its run's branch label, exactly equal to a
row's `branch` (`kubernetes.go:5396-5415`, looked up at `:5455`). One that matches no row goes to
one group titled *"no environment recorded"*, never `""` (task 08 rule 1b, review M5). The join is done **once, in the control
plane** (task 08): every `ReleaseRow` carries its matched row's name as `environmentRow`, and the
desktop never joins again (review I8).

**A git branch is never compared with an environment name (review b02, C2).** A row's `branch` is
Panacea's label — the environment key for every non-base row (`feat-x` for `feat/x`,
`projects.go:2109-2118`) — and a deployment's `branch` is a sanitised run label, written by the
API (`clients/pipeline.go:609`) or by the webhook-receiver for a push, which keeps `_`
(`webhook-receiver/pipeline/trigger.go:1270`; §12). The branch → environment mapping is Panacea's access handler's:
`GET P/branches/{branch}/access` returns `data.env` from `branchEnv` → `mapBranchToEnvWithProd`
(`handlers/branch_security.go:29-37`; `project_security.go:646`, `:668-677`), for **any** branch,
with no existence check. `Reader.EnvironmentFor` (tasks 01, 03) reads it, and the answer is joined
to rows with `SameEnvironment`. So **the environment for a first Ship** of a never-deployed branch is
knowable before the deploy, without re-implementing `sanitizeBranchEnv` (`projects.go:3814-3826`),
which the survey rightly forbids (a fourth copy to drift). Hub ask HA7-1 is withdrawn (§9).

**That mapping is not quite the deploy's (b02 round 2, R1).** `branchEnv` reads the production
branch from the raw `production-branch` label only (`branch_security.go:32`). The deploy resolves
it with `ResolveProjectProductionBranch` — label, then the `primary-branch` annotation, then
`"main"`, skipping a label whose value is `prod`/`production` (`clients/pipeline.go:233-237`,
`clients/kubernetes.go:10179-10204`), and the environments list does the same
(`kubernetes.go:5189`). For a project whose production branch is annotation-only or mislabelled,
the two disagree: annotation `primary-branch=develop` and no label → the route answers `develop`
for `develop`, which deploys to `prod`. Phase 7 copies no resolver to correct it. It detects the
disagreement from the fact the deploy wrote: a Hub release whose `environment` is not
`SameEnvironment` as its joined Deployment's `Environment` (the label, `pipeline.go:608`) is shown
as *recorded against another environment* and is never joined by the Hub's name (task 08, rule 5a).
It is a Panacea defect phase 7 lives with (§12), relayed to Panacea's owner.

**7P-9 — Owner-only controls are detected by asking the owner-only route, never by parsing group
names.** `GET …/branches/{branch}/auto-deploy` is `authedProjectOwner` (`routes.go:1308`): a
non-owner gets `403` from the middleware before the handler runs, an owner gets `200` or the
handler's `409 "branch has no environment yet"` (`projects.go:4132-4134`). The watcher records that
status as `ownerControls: "yes" | "no" | "unknown"`. Parsing `project-<name>-owner` out of `/me`'s
groups would be a second reader of a namespace whose own file says it has exactly one
(`models/project_groups.go:5`). Developer rights are **not** probed — no developer-only GET exists
in the routes phase 7 uses — so a viewer's Ship is refused by Panacea and rendered verbatim, after
the reservation is abandoned. A key restricted to one project (review I5, Q-P7-7) meets the same
routes: the probe always answers `403` (a scoped key is capped at Developer,
`models/models.go:547-559`), a *Read only* key's Ship, Roll back, Rebuild, Cancel and posture change
are each refused with Panacea's own sentence (all `authedProjectDeveloper`, `routes/routes.go:1286`,
`:1026`, `:1291`, `:1431`), and
`/me` and `/projects` carry no mark of the scope. So the owner-only and no-match sentences name the
key as well as the role (tasks 18 and 06), and nothing tries to detect the scope.

**7P-10 — Corporate TLS through the operating system's trust store; no "insecure" switch.**
`panacea-mcp` needed `PANACEA_TLS_INSECURE=1` (`src/index.ts:69-71`) because Node carries its own
CA list. Go's `crypto/x509` on Windows verifies through the Windows certificate store, where KPMG's
interception root is installed — **from knowledge, not tested here**. Task 02 writes
`cmd/versioncheck` and does not run it. The owner runs the one live `GET /api/v1/version` from Go
on this laptop at `LIVE-CHECK.md` step 1 (task 20, OD-12; review I9). `PANACEA_APP_PANACEA_CA_FILE` adds a PEM for
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
→ `404 deploy_not_found`; `409` → `409 deploy_conflict`; `402` → `409 deploy_conflict` too, with
its sentence (the deploy route's budget and risk gates, `projects.go:2856`, `:2916-2918` — a
refusal to abandon, not an outage; review I2); `400`/`422` → `422 deploy_rejected`. The
`message` is Panacea's sentence, trimmed to 500 bytes; **a `5xx` sentence is never shown** (Panacea
appends `err.Error()` to them, e.g. `projects.go:1529`), only logged. *"Waiting for team approval"*
is read from `GET /api/v1/me`'s `approved` field (`handlers/me.go:48-59`), **never** by matching the
403 sentence.

**7P-13 — Nothing Panacea says is persisted.** Deploy data is server truth held in the watcher's
memory; the desktop slice is not persisted (provenance contract A5, widened by ADR-H1 §5.4). The
only thing phase 7 writes to disk is the ambiguous-binding choice (7P-6), which is configuration,
not Hub or Panacea data. **No persist version is claimed** (ruling R3).

**7P-14 — SSE is relayed for one thing at a time, in our own frames.** The pipeline-log stream
(`routes.go:2283`) and the readiness stream (`:2286`) are opened by the control plane only for the
one run or environment the user has drilled into; a second replaces the first. The connector parses
Panacea's `data: <json>` frames into `deploy.StreamEvent`, and the control plane re-emits them as
`event: log | readiness | problem | end` frames of its own, so the desktop never learns Panacea's
stream shape and the lift does not touch it. The desktop reads them with `fetch` and a stream reader,
because `EventSource` cannot send the Hub's local caller key header (Hub contract §9.4).

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
| 8 | `GET P/security-status?env=&deployId=` | 1043 | member | data `SecurityStatus` (`models/security.go:205`; `handlers/security_status.go:204-254`); `deployId` is a PipelineRun name (`:211-214`); `env` is the deployment's `Environment`, sent whenever non-empty, because a missing `env` reads production (`:192-201`) and a branch release would show production's verdict (review C3) | 03, 07, 12, 20 |
| 9 | `GET P/environments/{env}/pods` · `/events` · `/diagnose` | 1148, 1150, 1151 | member | list of `PodStatus` · list of `NamespaceEvent` (`clients/pod_introspection.go:47-61`, `:95-104`; `handlers/pod_introspection.go:139`, `:199`) · data `DiagnoseResponse` (`handlers/diagnose.go:70-79`, `:157`); task 03 decodes each into its own DTO | 03, 12 |
| 10 | `GET P/pipelines/{run}/logs/stream` · `P/environments/{env}/readiness/stream` | 2283, 2286 | member | SSE, `data: <json>` frames (`handlers/readiness_stream.go:195`, `handlers/logs_stream.go:118-141`), parsed by the connector and re-emitted as **our** frames (task 12) | 12 |
| 11 | `POST P/deploy` | 1286 | developer + `RequireSectionWritable("service")` | `202` data `{project,status,pipelineRun}` (`projects.go:2997-3005`) | 04, 09 |
| 12 | `POST P/pipelines/{run}/cancel` | 1291 | developer | `200` data `{status: "cancelled" \| "already_completed", message}` (`projects.go:3668-3687`) | 04, 10 |
| 13 | `POST P/deployments/{id}/rollback` | 1026 | developer | `200` data `Deployment` with `rollbackWarmth`, `coldStartSeconds`; `404`, `400` (no image), `409` (image pruned) (`projects.go:1547-1640`) | 04, 10 |
| 14 | `GET` / `PUT P/branches/{branch}/auto-deploy` | 1308, 1309 | **owner** | data `{branch,enabled,projectDefault,effective,isProductionBranch}`; `409` when the branch has no environment (`projects.go:4114-4147`); PUT body `{"enabled": bool}` (`:4160-4167`) | 04, 11 |
| 15 | `GET` / `PUT P/branches/{branch}/access` | 1430, 1431 | member / developer | data `{branch,env,accessMode,ipWhitelist,passwordSet,bypassTokenCount}` (`handlers/project_security.go:668-677`); PUT `{accessMode, password?, bypassTokens?, rotateBypassTokens?}` (`:681-691`). The GET's `data.env` is Panacea's own environment name for **any** branch, before its first deploy (`branchEnv`, `handlers/branch_security.go:29-37`) — `Reader.EnvironmentFor` (review b02) | 03, 04, 07, 09, 11 |
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
   Phase 7 reads no transcript, so OD-8's adapter rule does not arise. Session attribution (task 19)
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

---

## 6. Configuration — every variable phase 7 adds

Read in `control-plane/cmd/api/config.go`'s one block, as the existing sixteen are
(`config.go:23-45`). None is a secret. Phase 2's rule stands: no `PANACEA_APP_*` variable is ever
silently ignored (`config.go:69-70`).

| Variable | Default | Meaning | Task |
|---|---|---|---|
| `PANACEA_APP_PANACEA_URL` | *(none)* | Panacea's **origin**, e.g. `https://dev.panacea.kpmg`. A value with a path, a query or a non-`https` scheme exits at start (7P-11). Unset: the connect form asks | 05 |
| `PANACEA_APP_PANACEA_CA_FILE` | *(none)* | a PEM bundle **added** to the system roots (7P-10). An unreadable file exits at start | 02, 05 |
| `PANACEA_APP_PANACEA_TIMEOUT` | `15s` | per-attempt timeout on every Panacea call. `POST …/deploy` itself waits up to 15 s inside Panacea (`projects.go:2985`), so the default is not lower | 02 |
| `PANACEA_APP_DEPLOY_POLL_INTERVAL` | `15s` | the watcher's cycle for every bound project | 07 |
| `PANACEA_APP_DEPLOY_ACTIVE_POLL_INTERVAL` | `3s` | the cycle for a project with a run this user shipped that is not terminal | 07 |
| `PANACEA_APP_DEPLOY_HISTORY_INTERVAL` | `60s` | the most often `GET …/deployments` is read when nothing else changed (7P-5) | 07 |

---

## 7. File map

New unless marked. `cp/` is `control-plane/`, `d/` is `desktop/src/`.

| Path | Task | What it holds |
|---|---|---|
| `cp/internal/deploy/deploy.go` | 01 | `Deployer` and its parts, the domain types, `ProjectRef` |
| `cp/internal/deploy/errors.go` | 01 | `Error`, `Kind`, `KindOf` |
| `cp/internal/deploy/validate.go` | 01 | `ValidCommit`, `ParsePostureMode` |
| `cp/internal/deploy/deploytest/fake.go` | 01 | `Fake`, an in-memory `Deployer` with an injectable clock |
| `cp/internal/deploy/deploytest/conformance.go` | 01; rows 03, 04 | `Harness`, `Seeder`, `Conformance(t, Harness)` |
| `cp/internal/deploy/seams_test.go` | 20 | `SEAMS.md` names only real tests and holds no `<…>` |
| `cp/internal/deploy/panacea/client.go` | 02 | `Config`, `TokenSource`, `Client`, `New`, `ParseOrigin`, `Version`, `BuildHTTPClient`, `do` |
| `cp/internal/deploy/panacea/errors.go` | 02 | `mapStatus`, `readProblem`: status → `deploy.Error` |
| `cp/internal/deploy/panacea/wire.go` | 03, 04 | the DTOs, each citing the Panacea line it mirrors |
| `cp/internal/deploy/panacea/reads.go` | 03 | `Whoami`, `Projects`, `Environments`, `Latest`, `Deployments`, `Run`, `Scans`, `Pods`, `Events`, `Diagnosis` |
| `cp/internal/deploy/panacea/acts.go` | 04 | `Ship`, `Cancel`, `Rollback` |
| `cp/internal/deploy/panacea/governance.go` | 04 | the nine `Governor` methods: auto-deploy, branch posture, project access, URL prefix |
| `cp/internal/deploy/panacea/stream.go` | 12 | `Stream` for the two SSE routes |
| `cp/internal/deploy/panacea/panaceatest/server.go` | 03, 04 | a fake `panacea-api` over `httptest`, serving the recorded bodies |
| `cp/internal/deploy/panacea/testdata/*.json` | 03, 04 | response bodies, one per route and state |
| `cp/internal/deploy/panacea/live_test.go` (`//go:build live`), `live_guard_test.go` | 20 | the read-only half of the owner's live check; the guard that keeps it tagged and secret-free |
| `cp/internal/deploy/connection/connection.go`, `current.go` | 05 | `Manager`: address, token in the secret store, status; `Current`, the delegating `Deployer` |
| `cp/internal/deploy/binding/binding.go`, `choices.go` | 06 | `Resolver`, `Binding`, `State`; the local choice file |
| `cp/internal/statepath/atomic.go` *(only if ruling R2's lift has not landed)* | 06 | the atomic write the choice file uses |
| `cp/internal/deploy/deploywatch/watcher.go`, `join.go` | 07 | `Watcher`, `Snapshot`, its cycle, the cursor; the deployment join |
| `cp/internal/deploy/deploywatch/view.go` | 08 | `View` assembly: environments, releases, pins, collisions, serving-after-rollback |
| `cp/internal/deploy/ship/ship.go`, `confirm.go` | 09 | `Orchestrator.Ship`, `Readiness`, `PinPreview`; the confirm retry loop |
| `cp/internal/deploy/ship/core.go`, `rerun.go` | 10 | `withReservation`, extracted from `ship.go`; `Rollback`, `Rebuild`, `Cancel` |
| `cp/internal/deploy/sessions/sessions.go` | 19 | sessions in a shipped range, from `Panacea-Session` trailers |
| `cp/internal/api/deploy_connection.go`, `deploy_errors.go` | 05 | `/v1/deploy/connection`; `writeDeployError`, the one error mapping |
| `cp/internal/api/deploy_binding.go` | 06 | `PUT /v1/projects/{projectId}/deploy/binding` |
| `cp/internal/api/deploy_view.go` | 08 | `GET /v1/projects/{projectId}/deploy` |
| `cp/internal/api/deploy_acts.go` | 09, 10 | ship, ship-readiness, rollback, rebuild, cancel |
| `cp/internal/api/deploy_governance.go` | 11 | nine routes: auto-deploy, access, posture, URL prefix |
| `cp/internal/api/deploy_drill.go` | 12, 19 | run progress, scans, pods, events, diagnose, the two streams, sessions |
| `cp/internal/api/api.go` *(modified)* | 05, 06, 07, 08, 09, 11, 12, 19 | `Deps` fields; one `register…` call per new route file |
| `cp/internal/api/sessions.go`, `cp/internal/httpx/origin.go` *(modified, only if Hub C2 has not built HA7-4)* | 07 | the cursor header on `GET /v1/sessions`; `Access-Control-Expose-Headers` |
| `cp/internal/api/routes_audit_test.go` *(modified)* | 05, 08, 09, 11, 12, 19 | one `catalogReadOnlyRoutes` line per `GET` |
| `cp/internal/httpx/problem.go` *(modified)* | 05 | the `deploy_*` codes; `upstream_unavailable` if no earlier task added it |
| `cp/internal/httpx/origin.go`, `origin_test.go` *(modified)* | 05 | the preflight: `PUT` in `corsMethods`, `Idempotency-Key` in `corsHeaders` (review C1; a declared amendment to phase 2 task 09 and Hub task 37, C3); two membership rows |
| `cp/cmd/api/config.go`, `main.go` *(modified)* | 05, 06, 07, 09, 19 | the variables of §6; the wiring |
| `d/deploy/types.ts`, `api.ts` | 13; 16, 19 | wire types of every route above; `DeployApi` over phase 2's `client.ts` request helper (16 adds `ShipReadiness`, 19 `ReleaseSessions`) |
| `d/deploy/deploySlice.ts`, `loader.ts` | 13; 16 | the store slice — **not in `partialize`**; `onDeployCursor`, `loadDeployView` (16 adds the readiness load) |
| `d/store/store.ts` *(modified)* | 13 | one import and one spread for the slice; `partialize` untouched |
| `d/store/selectors.ts` *(modified)* | 13 | `selectDeployView`, `selectReleasePinKeys` (R6's key source) |
| `d/api/problems.ts`, `d/api/client.ts` *(modified)* | 13 | `upstream-down` (R4) if absent; `PUT`, `headers`, the new `ProblemCode`s |
| `d/api/poller.ts` *(modified, only if Hub C2 has not built HA7-4's `onCursor`)* | 13 | the cursor edge |
| the four `classify` call sites *(modified, only if `upstream-down` was absent)* | 13 | `EmptyDevelop.tsx:57`, `SessionPane.tsx:48`, `sessionVerbs.ts:85`, `useCreateSession.ts:38` |
| `d/deploy/DeployLens.tsx`, `deploy.css` | 14; 16–19 | the lens frame and its connection states; 16–19 mount their parts in the lens, 17–18 style theirs in `deploy.css` |
| `d/deploy/ConnectPanel.tsx`, `BindingPanel.tsx` | 14 | address + token, write-only; the six unbound answers and the one choice |
| `d/deploy/EnvironmentCard.tsx`, `EnvironmentList.tsx`, `ReleaseTable.tsx`, `PinCell.tsx`, `usePinRef.ts`, `Banners.tsx` | 15 | the three facts, the other environments, the history, the pins, stale and collision |
| `d/components/common/StatusDot.tsx`, `common.css`, `d/styles/designsystem.test.ts` *(modified)* | 15 | the fact statuses; the `--drift` guard if phase 4 has not added it |
| `d/deploy/useHeldAct.ts` | 16 (or 18, whichever lands first) | the 5 s hold every act shares |
| `d/deploy/ShipBar.tsx`, `ReleaseActs.tsx`, `ActBanners.tsx` | 16 | Ship and Cancel run; rollback and rebuild for the drawer; the last act's outcome |
| `d/deploy/ReleaseDrawer.tsx`, `PinManifest.tsx`, `ScanList.tsx`, `DiagnosePanel.tsx`, `LogPane.tsx`, `drillStream.ts` | 17 | the manifest and the drill-ins; the window's one open stream |
| `d/deploy/GovernancePanel.tsx`, `PostureControl.tsx`, `AccessList.tsx`, `useGovernance.ts` | 18 | auto-deploy, access, posture, URL prefix |
| `d/deploy/ReleaseSessions.tsx` | 19 | the drawer's sessions block |
| `d/components/AppShell.tsx` *(modified)* | 14 | `case "deploy"` renders `DeployLens` |
| `docs/superpowers/plans/2026-09-23-phase-7-deploy-parts/SEAMS.md` | 20 | the seams of §10.2, as built, and the Hub asks' state |
| `docs/superpowers/plans/2026-09-23-phase-7-deploy-parts/LIVE-CHECK.md` | 20 | the owner's one live session (OD-12), with a column for what was seen |
| `docs/superpowers/plans/2026-09-23-phase-7-deploy-parts/panacea-metadata-ask.md` | 20 | the R7 draft, for the owner to send |

---

## 8. Tasks, dependencies and order

Each task is 1–3 hours for one implementer who has read this file and the part. Estimates assume
the named dependencies have landed or their fakes exist; where a fake is the Hub's, it is
`hubclient.NewFake()` from Hub task H0.

The phase splits as `dependency-graph-and-order.md` recommended: **7a, the read path**, needs
nothing from phase 3 and can overflow into any gap; **7b, the write path**, needs phase 3's remote
reading and push, because Panacea builds only pushed commits (7P-7 settles the inferred P3 → P7 edge:
`TriggerPipelineRun` clones `project.RepoURL`, `clients/pipeline.go:174`).

| # | Task | Half | Hours | Depends on (this plan) | Depends on (elsewhere) |
|---|---|---|---|---|---|
| 01 | The `deploy` domain: `Deployer`, types, errors, `Fake`, conformance skeleton | 7a | 2 | — | — |
| 02 | Panacea transport: address, bearer, timeouts, retries of reads, error mapping, TLS roots, `Version` | 7a | 2 | 01 | — |
| 03 | Panacea reads: `Whoami`, `Projects`, `Environments`, `Latest`, `Deployments`, `Run`, `Scans`, `Pods`, `Events`, `Diagnosis` + recorded bodies | 7a | 3 | 02 | — |
| 04 | Panacea acts and governance calls: `Ship`, `Cancel`, `Rollback`, auto-deploy, posture, project access, URL prefix | 7a | 3 | 02, 03 | — |
| 05 | The connection: write-only token route, status, config, the `deploy_*` codes; the preflight allows `PUT` and `Idempotency-Key` (C1) | 7a | 2 | 02, 03 | Hub C1 `credstore` (or its three-method subset, part 05) |
| 06 | The binding: local project → Hub project and → Panacea project, by repository | 7a | 2 | 03, 05 | phase 3 task 13 `githost.ParseRemote`; Hub H0 `hubclient.Projects` fake |
| 07 | The watcher: the cycle, the snapshot, the cursor header, the deployment join | 7a | 3 | 03, 06 | Hub H0 `hubclient.Releases` fake; Hub C2 cursor mechanism (HA7-4) |
| 08 | The view: `GET /v1/projects/{id}/deploy` — environments, releases, pins, collisions, serving after rollback, owner controls | 7a | 3 | 07 | — |
| 09 | Ship: reserve → deploy → confirm or abandon, preconditions, retries, the crash window | 7b | 3 | 04, 08 | Hub H0 fake (H5 for integration); phase 3 task 08 `remote.State` |
| 10 | Rollback, rebuild and cancel | 7b | 2.5 | 09 | — |
| 11 | Governance routes: auto-deploy, project access, branch posture, URL prefix | 7a | 2 | 04, 08 | — |
| 12 | Drill-in routes: run progress, scans for a release, pods, events, diagnose, the two SSE relays | 7a | 2.5 | 03, 08 | — |
| 13 | Desktop data layer: types, client, slice, cursor-driven fetch, `upstream-down`, R6 key selector | 7a | 3 | 08 | Hub C2 poller cursor (HA7-4); phase 4's key-source loader (R6); Hub C3 local key in `client.ts` |
| 14 | Desktop: the lens frame, its four connection states, and the binding | 7a | 2.5 | 05, 13 | Hub C3 local key in `client.ts` (the lens works without it) |
| 15 | Desktop: the environment card, the release table, the pin cell, stale and collision banners | 7a | 3 | 14 | phase 4's `Ref` cache (R6) |
| 16 | Desktop: the Ship bar and the held acts (ship, rollback, rebuild, cancel) | 7b | 3 | 15, 09, 10 | **hard, no fallback:** phase 3 task 18 (`pushBranch`, `addGoodTickListener`, `selectConvergence`, `useConvergenceLoader`: the Push button and readiness edge 4, review I7) and phase 3 task 12 (the server push), both merged before task 16 starts (re-review b04-1, M-1) |
| 17 | Desktop: the release drawer (manifest), scans, why it is down, the log pane | 7a | 3 | 15, 12 | phase 4 tasks 16–17's `ShippedManifest` and selectors, if merged (part 17 says what if not) |
| 18 | Desktop: the governance panel | 7a | 2.5 | 15, 11 (uses 16's `useHeldAct` contract; lands the file if 16 has not) | — |
| 19 | Sessions in a release, from `Panacea-Session` trailers | 7b | 2 | 08, 17 | phase 3 task 02 (`gitx`) and task 05 (the hook, `S3-P7.1`) |
| 20 | Seams, the live verification recipe (owner-batched), the Panacea metadata ask draft | — | 1.5 to write; the owner's session about 45 min | all | Hub H5 running for the Ship steps; the owner's session (OD-12) |

**Order.** 01 → 02 → (03 ∥ 05's config half) → 04 → 05 → 06 → 07 → 08 → (11 ∥ 12 ∥ 13) → 14 → 15 →
(17 ∥ 18) → *7b:* 09 → 10 → 16 → 19 → 20.

**Parallel lanes.** After 08, the control-plane routes (11, 12) and the desktop (13 onward) are
independent. 7b starts when phase 3's tasks 08 and 12 are merged; until then 09 and 10 can be built
and tested against fakes of `ship.BranchState` and `hubclient.Releases`, but are not merged. Task
16 also waits for phase 3's task 18 (its Push button and readiness edge 4 come from it; no
fallback, re-review b04-1 M-1).

**Total:** about 51 engineer-hours — roughly **7 working days for one engineer, 4 for two** (one on
the control plane, one on the desktop after task 08). Add a day if Hub C1/C2 have not landed and
their three small interfaces have to be stood in for (parts 05, 07, 13 say exactly how).

---

## 9. Hub contract asks

Read against `.claude/worktrees/hub/docs/superpowers/contracts/2026-09-23-hub-contract.md` at
`a6ccd77`. Each ask is the exact shape phase 7 needs. **None is built by phase 7**; where the plan
proceeds before an answer, it says what it does meanwhile.

**HA7-1 — withdrawn (review b02, C2). Phase 7 asks the Hub for nothing here.** It asked for
`environment: null` at reserve, set once at the deployment join, on the premise that for a branch
Panacea has never deployed *"no route reports [the environment] before the deploy"*. That premise
was false. `GET P/branches/{branch}/access` (`routes.go:1430`, member-readable, §4 row 15) returns
`data.env` from `branchEnv` → `mapBranchToEnvWithProd` — the access handler's mapping, for **any**
branch, with no existence check (`handlers/branch_security.go:29-37`; `project_security.go:646`,
`:668-677`). It reads the production branch from the label only, so it can disagree with the
deploy for an annotation-only or mislabelled project; 7P-8 says how phase 7 shows that, and §12
records it (b02 round 2, R1). (`GET …/auto-deploy`'s `409` for a branch with no environment,
`projects.go:4132-4134`, is a different route.) So task 09 step 4 reads the name with
`Reader.EnvironmentFor` before it reserves, and §7.9's required, non-null `environment` stands
unchanged. The old *meanwhile* joined `environments[].branch` to the git branch, and that field is
the environment key (`feat-x` for `feat/x`, `projects.go:2109-2118`), so it refused almost every
real branch as *"never been deployed"*. It is gone, with `deploy_environment_unknown` and
`EnvironmentOptional`. What is left is `409 deploy_environment_invalid`: Panacea's name breaks
§7.9's `[a-z0-9-]` rule, for example `release-1.2`. If the Hub stream has already built the
nullable field, phase 7 never sends `null`, and the field can stay or go at the Hub's choice.

**HA7-2 — clarify §7.9's rollback confirm.** §7.9 allows `{"deploymentId":"d-7a1"}` on a rollback
confirm *"when Panacea returns one"*. Panacea's rollback returns the **source's** id (7P-4), which
the source release already holds, so that confirm would be `409 conflict`. Asked: the sentence
becomes *"Rollback: `{}`. Panacea creates no deployment for a rollback."*, and the fake refuses a
rollback confirm carrying a `deploymentId` equal to `sourceDeploymentId` with `409 conflict` — so a
phase 7 regression fails a test. Phase 7 sends `{}` regardless.

**HA7-3 — a shared deploy binding on the project (non-blocking).** 7P-6 keeps an ambiguous
Panacea-project choice on each laptop. Asked, additive:

```jsonc
// Project gains:
"deployTarget": null | { "kind": "panacea", "origin": "https://dev.panacea.kpmg", "project": "engagement-portal",
                         "setAt": "…", "setBy": User }
// PUT /v1/projects/{projectId}/deploy-target   role A   body { "kind","origin","project" } → 200 Project
// DELETE /v1/projects/{projectId}/deploy-target role A → 200 Project
// Event project.deploy_target_set { "kind","origin","project" }
```

*Meanwhile:* the local file. When it lands, part 06's resolver reads the Hub first.

**HA7-4 — make the change-cursor mechanism of §9.3 carry more than one cursor.** §9.3 sets
`Panacea-Hub-Cursor` on `GET /v1/sessions` and adds it to `Access-Control-Expose-Headers`; task C2
changes `poller.ts` to read it. Phase 7 needs a second, `Panacea-Deploy-Cursor`, for the same
reason. Asked: C2 builds the mechanism once, generically —

```go
// control-plane/internal/api — a Deps field C2 adds:
// CursorSources are read on every GET /v1/sessions response; each sets "Panacea-<Name>-Cursor".
CursorSources []CursorSource

type CursorSource interface {
	CursorName() string   // "Hub", "Deploy" — one token of [A-Za-z]
	Cursor() string       // opaque; "" means "do not set the header"
}
```

```ts
// desktop/src/api/poller.ts — the hook C2 adds:
export type CursorListener = (name: string, value: string) => void;
// startPolling(client, store, onGoodTick?, onCursor?: CursorListener)
// onCursor is called once per header whose value differs from the last tick's.
```

— and lists every `Panacea-*-Cursor` in `Access-Control-Expose-Headers` (`origin.go`'s
`Middleware`, `:183`ff). *Meanwhile:* if C2 lands its single-header version first, part 07 adds the
second source and part 13 the second listener, as a declared amendment to C2's code.

**HA7-5 — a `kind` filter on the release list (non-blocking).** 7P-4 needs the newest confirmed
**rollback** per environment. `GET /v1/projects/{id}/releases?panaceaProject=&environment=&state=confirmed`
returns runs and rollbacks together, newest `reservedAt` first, 200 at most; the watcher filters. A
`&kind=rollback` filter would bound it. Additive.

**Satisfied by the contract as written — no ask:** reserve/confirm/abandon and their idempotency
(§1, §7.9); the pin rules including `inherited`, `source_unpinned`, `source_unknown` (§7.9);
`not_reserver` (a teammate cannot confirm my reservation); the deployment join and its uniqueness;
the `upstream_unavailable` relay (§4.3); the credential-store row for the `pnc_` token (§9.1); the
local caller key (§9.4); `release.*` events in the change feed (§8).

**HA7-6 — one Azure DevOps repo-key spelling (to phase 3 and the Hub; review I6, plan re-review
b05 R-1).** *This paragraph first said phase 7 "only ever compares keys it computed itself with
`githost` on both sides, so it is unaffected". That was false, and is withdrawn. Its round-1
replacement recommended that phase 3 change `RepoKey()`; that left out the evidence below, and is
withdrawn too.* Task 06 step 3 sends phase 3's `RepoKey()` to the Hub's `ProjectForRepo`, and the
Hub compares keys *"byte for byte"*, so the two must spell Azure DevOps alike or OD-4's Azure
DevOps Ship is off. The documents, cited by anchor because both sibling trees are committing now:
- **Phase 3 task 13**, the `RepoKey` doc comment: `azure-devops/<org>/<project>/<repo>`, lowercased,
  for the `dev.azure.com` and the `visualstudio.com` remote forms alike (feat/phase-3-git at 6a71014).
- **Phase 3's plan, the H-3 row**, still gives `<org>.visualstudio.com/<project>/<repo>`. Phase 3's
  own review **I10** (*"Hub ask H-3's repository key has two formats"*) rules the task-13 form the
  better one, *"because `dev.azure.com/<org>` and `<org>.visualstudio.com` are one organisation"*,
  and fixes the H-3 row to quote it. I10 is still open.
- **The Hub contract**, §7.4 `POST /v1/projects`, still gives the old regex
  `^[a-z0-9-]{1,50}\.visualstudio\.com/[^/]{1,64}/[^/]{1,64}$`, citing phase 3's plan as its
  source. Its own **H-3** row says `project_remotes.repo_key` is *"`githost.RepoKey` verbatim"*:
  the Hub meant to follow phase 3's key, and the regex copied the H-3 row that I10 calls wrong.
- **The Hub plan, correction A1** (feat/hub at 52f0773), replaces that regex with
  `^azure-devops/[a-z0-9-]{1,50}/[^/]{1,64}/[^/]{1,64}$`, refuses a non-lowercase key, publishes
  the rule as `hubapi.ValidRepoKey` and `HostOfRepoKey` (Hub task 01), applies it in the fake at
  `CreateProject` (Hub task 04 step 2), and folds A1 into §7.4 in Hub task 44a, before any code.
  Phase 4's HA4-6 was answered *"= A1"*.

**The two answers.** (a) Phase 3 changes `RepoKey()` to `<lower(org)>.visualstudio.com/…`, and the
Hub keeps the contract's regex. (b) The Hub **replaces** its Azure DevOps pattern with task 13's
form, as A1 does. It does not widen the pattern to take both: one spelling per repository, and
§7.4's uniqueness holds under either answer. **Recommended: (b).** It keeps a host-neutral key that
does not name the legacy `visualstudio.com` host for a `dev.azure.com` remote (phase 3's I10), it
changes no code anyone has planned (task 13 already spells it), and the Hub has already planned it.
Its cost is text in two documents: Hub task 44a rewrites the §7.4 regex, and phase 3's I10 fix
rewrites its H-3 row. (a) would keep the Hub contract as written, but would change phase 3's code,
contradict phase 3's own review, and undo the Hub's A1.

**Asked:**
- **Of phase 3:** fix review I10 by quoting task 13's form in the H-3 row, as I10 says. It is the
  only change left, and it should see this ask before it edits the H-3 row.
- **Of the Hub:** keep A1, and fold it into §7.4 in task 44a as planned.

**Phase 7 builds nothing that assumes either answer:**
- the key sent is `RepoKey()` verbatim, chosen in one function, `hubKey` (task 06). Both answers
  keep it verbatim, so neither changes `hubKey`.
- a miss sets `HubMiss "key_spelling"` only when `hubapi.HostOfRepoKey` refuses the key. Phase 7
  copies no pattern, so readiness, the Ship refusal and the lens line (tasks 06, 07, 09, 14) follow
  whatever rule the Hub builds, and name no host.
- `TestHubBindsADOWithPhase3sKey` (task 06) seeds the Hub fake, through `CreateProject`, with the
  real `githost`'s own `RepoKey()`, never a literal, and must bind. It passes under either answer and
  goes red only if phase 3's key and the Hub's rule disagree. **HA7-6 is closed when it passes
  against the built code** (task 20).
- Under (a), the two Azure DevOps literals in `TestHubMissNotLinked` flip. Nothing else in phase 7
  changes.

**Status:** the plans agree, so HA7-6 no longer blocks OD-4, but it stays open until the binding
test passes against built code. *If the built code disagrees:* GitHub repositories still Ship;
Azure DevOps repositories bind to Panacea and read their deployments, but Ship is off with the
cause named.

---

## 10. Asks of, and seams with, the other phases

### 10.1 What phase 7 consumes

| From | Seam | Used in | If it is not there |
|---|---|---|---|
| Phase 2 | `routes.Handle` recorder (`api.go:174-184`); `catalogReadOnlyRoutes` (`routes_audit_test.go:32`); `httpx.WriteProblem` (`problem.go:93`); `statepath.Dir` (`statepath.go:30`); `LensPlaceholder` for `deploy` (`AppShell.tsx:60-67`, already `phase={7}`) | every task | — (all exist) |
| Phase 2 / R2 | the shared `writeAtomic` (ruling R2 lifts it into `statepath`) | 06 | if phase 4 has not lifted it yet, task 06 does the lift, as R2 describes, **with its tests** |
| Hub H0 | `hubapi`, `hubclient.Releases`, `hubclient.Projects`, `NewFake()` (contract §9.2); `hubapi.HostOfRepoKey` (Hub task 01, correction A1, in S0) | 06–10 | nothing phase 7 can compile against; H0 is first in the Hub's order (ADR-H1 §5.9) |
| Hub C1 | `credstore.Secrets` (§9.1) | 05 | part 05 declares `connection.Secrets` — `Get`, `Put`, `Delete`, the same signatures — which `credstore.Secrets` satisfies, and wires `credstore.Memory()` in tests |
| Hub C2 | the relay and the cursor mechanism | 07, 13 | HA7-4's *meanwhile* |
| Hub C3 | the local caller key on every request | 14 | the lens works without it; the §6.6 exposure is C3's to close |
| Phase 2 task 09; Hub C3 (task 37) | the preflight constants in `origin.go:168-172`, which both already write | 05 | **amended by phase 7:** `PUT` and `Idempotency-Key` are added. Whichever of task 05 and Hub task 37 lands second adds its entry to the list it finds and does not paste its own literal; task 05's membership rows go red if an entry is dropped (review C1) |
| Phase 3 task 08 | `remote.State{Upstream, Ahead, Behind}`, `Convergence.Tip` | 09 | 7b does not merge |
| Phase 3 task 12 | `POST …/push` | 16 (offered beside a disabled Ship) | Ship says *"push first"* without the button |
| Phase 3 task 13 | `githost.ParseRemote`, `Remote.RepoKey()` | 06 | 7a's binding cannot match; the connect screen asks for the Panacea project by name instead (part 06). **Plus HA7-6 (§9):** `RepoKey()`'s Azure DevOps spelling must agree with the rule the Hub builds (`hubapi.ValidRepoKey`; the Hub's correction A1 takes task 13's `azure-devops/…` form), or no Azure DevOps repository finds its Hub project and Ship stays off with the cause named (`HubMiss "key_spelling"`, part 06) |
| Phase 3 task 05 | the `Panacea-Session` trailer on agent commits (3-D6, `S3-P7.1`) | 19 | the manifest says *"sessions are not recorded for commits made before phase 3"* |
| Phase 4 | the key-source loader (R6) and the `Ref` cache it fills; Discover opening a past revision read-only (provenance contract §4.1) | 13, 15, 17 | the pin cell renders `resolving` for a key it cannot resolve — never a head — and part 15 states it |
| Phase 5 | an approved `design:` key for the branch, to pass as `designKey` at reserve | 09 | Ship reserves with `designKey: null`; the Hub never infers one (§7.9) |

### 10.2 What phase 7 leaves for later phases — the seams

| Seam | Where | The later phase | Must not |
|---|---|---|---|
| **S7-N1** the native deployer | `deploy.Deployer`; `deploytest.Conformance` | the Panacea lift (`OD-13`): a native implementation passes the suite and replaces `panacea.Client` in `main.go`; the `PanaceaApp/panacea/<host>` credential row is deleted by that task | change a domain type to fit Panacea's wire shapes; add a Panacea-specific method to `Deployer` |
| **S7-N2** the metadata mirror | `panacea.Client.Ship` sends no `metadata` today | if R7's ask is granted: the client adds `metadata: {"panacea.app/release": "<rl-…>"}` and the watcher may confirm a crashed reservation by the run's annotation (ADR-H1 R3) | pin by that annotation — it names a reservation, and the pin is still the Hub's |
| **S7-N3** Panacea-side hash suffix | `D2` (c) | if Panacea appends a hash when truncating, the collision state in task 08 stops firing for long names that differ past the 20th character; a push-and-Ship label split of one branch (`feat_x` / `feat-x`, §12) and `/`-versus-`_` twins are untouched by a hash on truncation | re-implement `sanitizeBranchEnv` in phase 7 |
| **S7-N4** a rollback as a Panacea row | "serving after rollback" in task 08's view (7P-4) | if ask 2 of `panacea-metadata-ask.md` is granted: a portal rollback shows as a `rollback` row; the Hub's rule stays for cockpit rollbacks | drop the Hub's confirmed-rollback reading before Panacea's rows are observed live |
| **S7-E1** phase 8's environment | the control plane's Panacea address and CA are configuration (§6) | phase 8 sets them in the product's environment; nothing in phase 7 assumes a laptop | hard-code `dev.panacea.kpmg` |
| **S7-X1** another deploy target | `deploy.Deployer` | a customer-hosted tier that deploys elsewhere implements the interface | branch on target type above the interface |
| **S7-P1** the criterion evaluator | the manifest's criteria render `open` + *"no evidence recorded yet"* | phase 3's `evidence.Evaluator` at `Deployment.Commit` (`S3-P7.5`) | evaluate at the branch head (P12) |
| **S7-P2** the merge link | task 17's `PinManifest` passes no `mergeRef` | whichever phase records a merge link in the Hub adds a field to the release or a route; `PinManifest` passes it | infer a merge from a commit being on `main` |
| **S7-S1** sessions by trailer | task 19's `sessions.Reader.Read` | phase 3's hook stays the only writer of `Panacea-Session` (`S3-P7.1`) | infer a session from a commit's time |

Task 20's `SEAMS.md` restates these rows with the file, line and test that hold each one, as built.

---

## 11. Open questions for the owner — each with the recommendation the plan builds on

| # | Question | Recommendation (what is built) |
|---|---|---|
| **Q-P7-1** (A-P7-1) | May *"redeploy"* become **"Rebuild <version>"**, which rebuilds that release's exact commit and keeps its brief pin, instead of Panacea's redeploy, which rebuilds whatever the branch holds now (7P-3)? | **Yes.** The alternative makes an inherited pin false whenever the branch has moved |
| **Q-P7-2** | Send Panacea's team the **metadata ask** (R7), and in the same note ask them to **record a rollback as a deployment row**, so a rollback made in their portal is visible to us (7P-4)? | **Yes, now** — both have another team's lead time. Task 20 writes the note; sending it is yours |
| **Q-P7-3** | Ask Panacea to **append a short hash** when it truncates a long branch name to an environment (brief `D2` c)? | **Yes**, in the same note. Until then the cockpit shows a collision after the fact (task 08) |
| **Q-P7-4** | The branch posture offers **`none` — public on the internet** — to any developer, because Panacea's API does (`routes.go:1431` is developer-gated). Should the cockpit offer it? | **Yes, as Panacea does**, held for 5 s with the consequence printed first: *"Anyone on the internet will be able to open this branch."* Overrule to hide `none` and `password` from the cockpit |
| **Q-P7-5** | A rollback made **outside the cockpit** cannot be seen (7P-4). Acceptable for v1, said in the lens's help text? | **Yes**; Q-P7-2 is the fix |
| **Q-P7-6** (b02 round 2, R1) | Ask Panacea to make `branchEnv` (`handlers/branch_security.go:29-37`) resolve the production branch with `ResolveProjectProductionBranch`, as its deploy and its posture path already do? Until then, for a project whose production branch is annotation-only or mislabelled, Ship records the Hub release against the access route's answer and the lens flags it after the deploy (7P-8, §12) | **Yes**, in the same note as Q-P7-2 and Q-P7-3. Phase 7 builds the after-the-fact flag and corrects nothing; a Hub-side correction at the deployment join would be a new Hub ask, and is not built |
| **Q-P7-7** (review I5) | Which `pnc_` key does the connect panel tell people to mint: **unrestricted** (*No restriction — full access, same as you*), or one **restricted to a project at *Deploy***? A restricted key sees one project (`clients/kubernetes.go:1237-1249`), is capped at Developer (`models/models.go:547-559`), so every owner control reads *"no"*, and a *Read only* one — the portal's default once a project is picked — can only read: it cannot Ship, roll back, rebuild, cancel or change who can open a branch (`middleware/rbac.go:265-276`; `routes/routes.go:1286`, `:1026`, `:1291`, `:1431`). The connection holds **one** key, so a restricted key means one project per cockpit. **What the recommended key costs:** an unrestricted key carries every right its holder has, on every project — platform admin included, for an administrator (`IsSuperAdmin` reads the holder's own groups, `models/models.go:218-228`). Phase 7 uses at most owner rights, and only on the bound projects; no route it calls needs admin rights. A third option, one restricted key per bound project, is **not built**: the connection holds one key | **Unrestricted.** The lens spans every local repository's project and draws the owner controls, and the key is the user's own, in Credential Manager, never in a sandbox, revocable in the portal. Built behind one constant (`KEY_ADVICE`, task 14); either answer leaves the no-match and owner-only sentences true, because both already name the key (tasks 06, 18). Overrule to restricted-at-*Deploy*: change `KEY_ADVICE`, and accept one project per cockpit and no owner controls |

**What the owner provides, and when (OD-12, batched to the end):** one `pnc_` token minted in the
Panacea dev portal (Account → API keys, with no project restriction unless Q-P7-7 is overruled), pasted into the connect panel during the hands-on session;
one dev Panacea project whose repository is also a Hub project; the owner's eyes on the lens.
Nothing blocks development: every task runs against `deploytest.Fake`, `hubclient.NewFake()` and
recorded Panacea bodies.

---

## 12. What this plan did not verify

- **Nothing was compiled or run.** No Go, no TypeScript, no test.
- **No network call was made by this pass.** The Panacea dev address and version are the
  orchestrator's measurement (`orchestrator-rulings.md`, *Settled by measurement*). No token was
  used and no response body was observed; every Panacea shape is read from Go source at
  `406d0da95` (§2), and a later Panacea commit may move any of it. Task 20's `LIVE-CHECK.md`
  re-checks against the live dev API, in the owner's session (OD-12).
- **Go on Windows trusts KPMG's interception root through the system store** (7P-10) — from
  knowledge; `LIVE-CHECK.md` step 1 (task 20) has the one live call that settles it. Task 02 only
  writes the command (review I9).
- **Whether `%2F` in `GET P/branches/{branch}/access` reaches Panacea's handler as `/`.** Panacea
  reads the segment with `r.PathValue("branch")` (`project_security.go:636`), and the Go 1.22+
  `ServeMux` unescapes a wildcard segment (per the `net/http` documentation; not run here). The ingress in front of it (Traefik) was not checked
  for decoding or rejecting `%2F`. Task 20's live check settles it for a `feat/x` branch. If it
  fails, `EnvironmentFor` answers with Panacea's error — a 404 on this route is rewritten to
  *"Panacea could not read this branch name. It may contain `/`…"* (task 03) — Ship refuses with
  `409 deploy_environment_unreadable` (task 09), the watcher keeps the refusal per branch and asks
  again once a minute (task 07), and the card says so (task 15). Nothing guesses (b02 round 2, R2).
- **Panacea's access route and its deploy disagree for some projects (b02 round 2, R1) — a
  Panacea defect phase 7 lives with.** `branchEnv` (`handlers/branch_security.go:29-37`) reads the
  production branch from the raw label (`:32`); the deploy (`clients/pipeline.go:233-237`) and the
  environments list (`clients/kubernetes.go:5189`) use `ResolveProductionBranch`
  (`kubernetes.go:10179-10204`: label, then `primary-branch` annotation, then `main`, skipping a
  `prod`/`production` mislabel). Panacea fixed the same class for posture
  (`handlers/posture_env_prodbranch_test.go`) but not here. Phase 7 shows the disagreement after a
  deploy (task 08, rule 5a) and does not correct it. How many live projects are annotation-only or
  mislabelled was not counted; `LIVE-CHECK.md` step 5a (task 20) compares the two answers for the live
  project. The fix is Panacea's (Q-P7-6).
- **Whether `developer` is accepted as a role spelling on `POST …/access`** — task 04 sends `editor`,
  the spelling `access.go` produces (`middleware/rbac.go:219`).
- **Whether `GET /api/v1/projects` returns `repoUrl` to a non-admin token.** The `List` handler
  (`projects.go:746-866`) was read for its envelope, not for per-field redaction. If it omits the
  field, part 06's fallback applies (the user names the project once).
- **That no rollback row appears anywhere in Panacea.** Read from the handler, the client and every
  `Trigger` assignment (7P-4); not observed live.
- **Rate limiting.** `panacea-api` applies `middleware.RateLimit` to one route only
  (`routes.go:2271`, the redeem limiter). An edge limit in front of it, if any, was not checked. The
  watcher's worst case is ~3 requests per bound project per 3 s while a run is active.
- **The Hub's code does not exist.** `hubclient`, `hubapi`, `credstore` and the fake are read from
  the Hub contract at `a6ccd77`; their final spellings may differ, and parts 05–10 name the exact
  methods they call so the difference is a compile error, not a silent one.
- **Phase 3, 4 and 5 interfaces** are read from their plans (phase 3 at its worktree's current text;
  phase 4's 2026-09-23 plan was still a stub when parts 13 and 15 were written, so R6's loader there
  is taken from the ruling and the provenance contract; phase 5's plan was not found to name
  `upstream-down`). Phase 4's parts have since been written: part 17's `ShippedManifestProps` and
  the phase 4 selector names it uses were checked against phase 4's tasks 16 and 17 at the phase-4
  worktree's `e330498` on 2026-09-24. Parts 13 and 15 were not re-checked against them.
- **How the lens looks.** Visual acceptance is the owner's eyes (OD-12); every UI task says so.

### Settled from source, no longer open

- **`Deployment.branch` is a label value, never a git branch (review I4, settled 2026-09-24;
  second writer added by re-review b03 round 1, I-1).** Panacea has **two** writers of the label,
  and they disagree:
  - **The API** (Ship, Rebuild, a manual deploy) stamps `panacea.io/branch:
    sanitizeLabelValue(branch)` (`clients/pipeline.go:609`). That function turns `/` and `_` into
    `-`, drops every other character outside `[a-zA-Z0-9.-]`, cuts at 63, trims `-` and `.`, and
    answers `"unknown"` for an empty result (`:2351-2364`).
  - **The webhook-receiver** (push-to-deploy, which phase 7 turns on: §4 row 14, tasks 11 and 18)
    stamps `panacea.io/branch: sanitizeLabel(event.Branch)` (`services/webhook-receiver/pipeline/
    trigger.go:1270`). That function turns `/`, `\`, `:` and space into `-`, **keeps `_`**, drops
    every other character outside `[a-zA-Z0-9._-]`, trims every non-alphanumeric end, cuts at 63,
    and answers `""` for an empty result (`:1553-1574`, `truncateLabel` `:1546-1551`). Its run
    carries `panacea.io/project` (`:1268`), so the deployments list shows it
    (`clients/kubernetes.go:5599`). Its environment is `branchEnvName`, which does turn `_` into
    `-` (`models/event.go:136-148`).

  So one branch `feat_x` can appear in environment `feat-x` under two labels: `feat_x` (a push)
  and `feat-x` (a Ship). The deployments list reads the label back verbatim
  (`clients/kubernetes.go:5750`). No annotation holds the real branch. So: `branchesSeen` and the
  collision banner compare and show labels as Panacea wrote them, never normalised (a fourth copy
  of Panacea's rules is exactly what this plan refuses); the banner counts **labels**, never
  branches, and says a push and a Ship write the label differently (task 08 rule 9, task 15);
  Rebuild takes its branch from the source's Hub `branchRef` only and refuses
  `no_branch` without one — the `source.Branch` fallback is gone (task 10, review M23); nothing
  sends a `Deployment.branch` as a branch or a `branchRef`. Because Rebuild never sends a label,
  Panacea's `environmentForBranch` (`branch == prodBranch`, `pipeline.go:2195-2207`) never sees
  `release-x` for a production branch named `release/x`. Residual: an environment's label for a
  branch cannot be matched to a git branch without a copy of Panacea's rule, so the cockpit never
  claims which branch is serving (task 15).
- **`security-status` must be sent the deployment's environment (review C3, settled 2026-09-24).**
  `resolveEnv` reads a missing `env` as `"production"` (`handlers/security_status.go:192-201`),
  `tenantNamespaceForEnv` maps `""`, `prod` and `production` to the base namespace
  (`handlers/tier_store.go:106-113`), and `collectScansFromK8s` keeps only ConfigMaps whose
  `panacea.io/env` matches (`security_status.go:526-534`). `Scans(ctx, p, env, deploymentID)`
  (task 01) sends `env` whenever it is non-empty (task 03), the watcher passes the newest
  deployment's `Environment` (task 07), and the drill route requires `environment` (tasks 12, 13,
  17), as Panacea's portal does (`SecurityScanReview.tsx:437-440`). Not run live.
- **Panacea's 402 on `POST …/deploy` is a refusal with a sentence (review I2, settled from source
  2026-09-24).** `TriggerDeploy` (`handlers/projects.go:2942`) runs the budget gate (`:2962` →
  `:2856`, *"new deploys are paused: "* + a reason from `handlers/budget_resolve.go:229-245`) and
  the risk gate (`:2979` → `:2916-2918`). They are the only 402 writers on routes phase 7 calls. The
  other two, `handlers/web_search.go:604` and `:752`, are AI routes. Task 02 maps 402 to
  `KindConflict` with the sentence, so task 09 abandons the reservation with Panacea's reason. The
  posture gate composed after the risk gate answers **403** (`project_security.go:155-157`), so it
  is `KindForbidden`. Neither 402 was observed live.
- **The value of `panacea.io/environment` for the production environment is `"prod"` (review M5,
  settled from source 2026-09-24, Panacea `b02424691`).** The API's deploy writes
  `environmentForBranch` (`clients/pipeline.go:237`, `:608`, the function at `:2195-2207`), a push
  writes `TargetEnvironment` (`webhook-receiver/models/event.go:116-129`, `pipeline/trigger.go:760`,
  `:1271`), and both answer `"prod"` for the production branch. The advisory run's `TargetEnv` label
  (`pipeline.go:842`) never reaches the deployments list (`clients/kubernetes.go:5633-5642`). A run
  with **no** label is legacy: it reads back `""` (`:5681`) and is joined by its branch label as
  Panacea's environments list does (`:5396-5415`, `:5455`), else shown under *"no environment
  recorded"* (7P-8, task 08 rule 1b). Not run live: task 08 still surfaces any other spelling under
  `unmatched`, and task 20 records the live names.
- **HA7-6 — the Azure DevOps repo-key split (review I6, plan re-review b05 R-1).** Read from plan,
  review and contract text, not code, and cited by anchor with the commit because both sibling
  trees move: phase 3 task 13's `RepoKey` doc comment, its plan's H-3 row and its review's I10
  (feat/phase-3-git at 6a71014); the Hub contract's §7.4 `POST /v1/projects` and H-3 rows, and the
  Hub plan's correction A1, task 01's *"repository keys"* block and task 04 step 2 (feat/hub at
  52f0773). The Hub fake applying `hubapi.ValidRepoKey` at `CreateProject` is read from Hub task 04
  step 2's text. Not checked: whether either sibling's code exists or matches its text; and
  whether a `DefaultCollection` remote keeps the same project segment.




