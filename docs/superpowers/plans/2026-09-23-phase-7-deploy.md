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
against**. Cancel, rollback and redeploy are the three other acts; auto-deploy, project access,
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



