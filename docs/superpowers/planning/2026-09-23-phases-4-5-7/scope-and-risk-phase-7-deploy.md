# Scope and risk — Phase 7, the Deploy connector (product journey: **Deploy**)

Written 2026-09-23 for the product owner's review. **Not an implementation plan** — `CLAUDE.md`:
*"The product owner reviews prototypes before implementation plans, and plans before code."*
Companion prototype: `prototype/phase-7-deploy.html` (one self-contained file, open by double-click).

## 0. The decisions you must make before a plan can be written

Three are new to this brief. Three more are already on your desk in the provenance contract and
also gate phase 7; they are listed after, not re-argued. Section 0.1 lists what this brief
**asserts** — overrule any of them and the named sections change.

### D1 — Is a release's provenance visible to the team, or only to the person who shipped it?

**New finding, inferred rather than read in code.** The provenance ledger and the brief store both
live in the control plane's **local state directory** (contract §2.1: `statepath.Dir("provenance")`,
`statepath.Dir("discover")`), and the control plane is a loopback process on each user's machine
(contract §6.6: `127.0.0.1:8730`; `PROGRAM.md`: local-first). So:

- A release **a teammate** ships from **their** cockpit has no release link in **my** ledger. On my
  screen it reads *"no brief is recorded for this release"* — the same words as an auto-deploy.
- Even if the metadata ask is granted and the brief key rides on the PipelineRun, my cockpit would
  resolve a key from **their** brief store, which it does not have. The contract's only word for that
  is `missing` — *"data loss in our own store"* — which would be false.

The spine promise — *"attached to the release in Deploy"* — therefore holds today **per person,
not per project**.

| Option | What the team sees | Recommendation |
|---|---|---|
| **a. Per person, for phase 7, said out loud.** Releases from other cockpits read *"no brief is recorded for this release"*; the help text says why | Only their own releases carry a brief | **Recommended for phase 7** — it is true, and cheap |
| b. Metadata ask **plus** a shared brief store | Every release shipped from any cockpit carries its brief | The right end state. It is a **hosting** decision (where Discover and the ledger live), not a phase 7 task, and it needs the Panacea ask granted |
| c. Metadata ask alone | Keys without text: every teammate's release reads `missing` | **Not recommended.** It trades a true "none" for a false "data loss" |

If you choose (a), the contract's `none` wording stands; if (b), phase 7's plan waits on a hosting
decision and the contract's `PipelineRun annotation mirror` row changes from *recovery only* to *a
source*.

### D2 — "Arbitrary names": what does the cockpit do about the 20-character collision, and is the URL prefix the answer?

The environment is **derived** from the branch name and truncated at 20 characters; two long
branches silently share one namespace (R2). The only name a user actually chooses is the
owner-only, globally unique **URL prefix** (§2.6).

| Option | Cost | Recommendation |
|---|---|---|
| **a. Show collisions honestly, after the fact** — when an environment's newest release names a different branch, both branches say so | Small. Cannot prevent the overwrite, only reveal it | **Always build this** — it is the floor |
| b. Refuse, at branch creation, names that would exceed the platform's 20 characters | Prevents most cockpit-made collisions. Copies the length rule — a fourth thing to drift, which survey §3.2 warns against | Only if the platform will not fix it |
| **c. Ask Panacea to append a short hash when truncating** | The right fix. Long-named branches change namespace on their next deploy — a migration on someone else's lead time | **Recommended to raise now**, alongside (a) |
| **URL prefix in phase 7?** Offer it (owner-only) as "the name people type" | One control, one route, first-come conflicts shown verbatim | **Recommended: in scope.** It is the honest reading of "arbitrary names" |

### D3 — "Which sessions produced this release": what fact does it stand on?

The phase brief asks for it; no link kind in the provenance contract carries it (§4.2, R4).
`AGENTIC-UX.md` §2 property 7: *if you cannot name the fact that justifies an element, the element
must not exist.*

| Option | Fact it stands on | Recommendation |
|---|---|---|
| **b. Phase 3 stamps a session id on every agent commit** (a trailer, e.g. `Panacea-Session: <id>`); the manifest lists the sessions whose commits are in the shipped range — the previous release's commit on that environment up to `Deployment.Commit` | The commits themselves. Commits made outside any session say so | **Recommended.** Adds one obligation to phase 3 (contract §8.3) |
| a. Drop sessions from the release in phase 7; show brief, design, merge, commit, image | Nothing to stand on, so nothing drawn | What phase 7 ships if phase 3 declines (b) |
| c. Snapshot the sessions registered on the branch at Ship | The registry at one instant. Includes sessions that contributed nothing or were abandoned, so the label "produced" would be false | Not recommended |

The prototype draws option **b**, labelled as depending on D3.

### Already asked in the provenance contract — they gate phase 7 as well

- **Contract Q6 — who owns control-plane authentication, and does phase 7 wait for it?** Gates the
  first task, reads included (R1). This is the most urgent of the six.
- **Contract Q1 — is the Ship-time brief head the right pin?** The prototype assumes yes and shows
  the merge-reviewed revision beside it when they differ.
- **Contract Q2 — releases shipped outside the cockpit** (auto-deploy, the portal): is *"no brief
  is recorded for this release"* acceptable? The prototype assumes yes. Note the consequence: with
  auto-deploy on, **every** push produces an unpinned release. D1 adds a teammate's cockpit to
  this list.

### 0.1 Asserted by this brief — overrule any of them

| # | Assertion | Sections that change if overruled |
|---|---|---|
| **B1** | **Access is two controls**, drawn as the API offers them: project groups and roles, plus a per-branch posture. The phase brief's *"access control through EntraID groups"* per branch does not exist (§2.4, R3). Per-branch group binding, if you want it, is a request to the Panacea platform — cross-repo, security-sensitive because it changes ForwardAuth — and phase 7 does not wait for it | §2.4, §3, prototype "Who can reach it" |
| **B2** | **Ship sends no scan-mode overrides.** The project's modes apply; the lens shows each scan's mode | §3, prototype "Scans" |
| **B3** | **Ship, rollback and access changes are optimistic with a five-second undo window**, not confirm dialogs (`DESIGN-SYSTEM.md` interaction standards). What Ship will build and pin is printed beside the button *before* it is pressed | prototype top bar |
| **B4** | **Times are absolute** (`Deployment.createdAt`), never *"2m ago"* (`AGENTIC-UX.md` §2 property 1). A per-user read cursor would allow *"since you last looked"*; phase 2 was not checked for one | prototype releases |

## 1. What this phase is

A branch in the cockpit gets a **Ship** action. Ship starts a real Tekton pipeline on Panacea
through the existing `panacea-api`, and the Deploy lens becomes the honest account of what
happened: which commit, which image, which environment, whether the build passed, whether the
scans passed, whether pods are actually serving — and **which brief revision the release was
shipped against**.

It is the last rung of the provenance thread (`CLAUDE.md`: *"attached to the release in
Deploy"*). It is **not new platform work**: every capability the phase brief names already exists
as a `panacea-api` route (`survey-phase-7-deploy.md` §2). The work is a typed Go client in the
control plane, a polling loop on the existing heartbeat, a local release ledger, and the lens.

Inputs read for this brief, in order: `survey-phase-7-deploy.md`; `provenance-contract-draft.md`
(§1–§7); `docs/DESIGN-SYSTEM.md`; `docs/AGENTIC-UX.md` §2 and §4; `prototype/app.html` (style block
and the old `viewDeploy`). Several facts were re-read from Panacea source for this brief, because
they change the design; §6 lists each.

**One thing the old prototype got wrong, which this brief corrects.** `prototype/app.html`'s
Deploy lens has an *"Environment name — any name you like"* field. The platform does not work that
way: the environment is **derived** from the branch name and truncated to 20 characters (§5, R2).
The closest real thing to "a name you choose" is the branch **URL prefix** (§2.6), which the
survey did not cover.

## 2. In scope

Every row names the route it stands on. Routes and guards are from `survey-phase-7-deploy.md` §2
unless marked.

### 2.1 Connect — once, per user

| What | Stands on |
|---|---|
| Paste a personal `pnc_` token once, into a **write-only** control-plane endpoint; nothing ever returns it | `middleware/api_key.go` (survey §5.4); contract §6.6, A4 |
| Store it in the OS credential store | **new dependency** — neither `control-plane/go.mod` nor `Cargo.toml` has one (contract §6.6) |
| Base URL is configuration, never hard-coded | two written sources contradict (survey §5.5) |
| Render the pending-approval 403 verbatim: *"no access yet: your account is waiting for team approval"* | `RequirePortalGrant`, `rbac.go:374` |
| Onboarding copy says where the token comes from: portal → Account → API keys | the app **cannot** mint one: `POST /api/v1/me/api-keys` needs a browser session (survey §5.4) |

### 2.2 See — environments, releases, build, serving, scans

| What | Stands on |
|---|---|
| Environments the project really has | `GET …/environments` — live namespaces, not a preset triple |
| Git branches, as a **separate** list, never merged with environments | `GET …/branches` lists remote git branches |
| Lens header: newest release per environment, with serving state | `GET …/deploys/latest` — polled on the one heartbeat |
| Release history | `GET …/deployments` — union of live PipelineRuns and the durable store |
| **Build** and **serving** as two facts. `Status` is the pipeline; `LiveReadyReplicas` is the pods: `nil` = not measured, `0` = measured and down, `>0` = serving. Only the newest row per environment carries it | `models.Deployment` (survey §3.3) |
| `Version` empty renders as *"version unknown"*, never blank | same |
| Scans for **this** release: `Overall`, `Blocking[]`, `Incomplete[]`, per-scan `status` / `mode` / `skipReason` / `errorReason`, and whether findings are attributable (`detailsSource`) | `GET …/security-status?deployId=` — **re-read from `models/security.go` for this brief** |
| Pipeline progress while running: `currentStep`, `completedSteps`, `failedStep`, `errorMessage` | `GET …/pipelines/{run}` — **re-read from `models.go:2386–2397`** |
| Drill-in for one environment only: live log follow and live ready count | the two SSE routes — opened **only** for the environment the user drilled into, never one per row |
| "Why is it broken": pods, events, diagnose | `…/environments/{env}/pods · /events · /diagnose` |

### 2.3 Act — ship, cancel, roll back, redeploy

| What | Stands on | Guard |
|---|---|---|
| **Ship** this branch at a named commit | `POST …/deploy` → 202 with the run name, synchronously | developer |
| Cancel a running pipeline | `POST …/pipelines/{run}/cancel` | developer |
| Roll back an environment to a previous release | `POST …/deployments/{id}/rollback` | developer |
| Redeploy an environment | `POST …/environments/{env}/redeploy` | developer |

Rollback and redeploy **inherit the pin of the release they re-run**, never today's brief head
(contract §3.4).

### 2.4 Govern — auto-deploy and access, drawn as the API really offers them

| What | Stands on | Guard |
|---|---|---|
| Auto-deploy on push, per branch | `GET` / `PUT …/branches/{branch}/auto-deploy` | **owner** — a developer sees it disabled, with the reason |
| **Project** access: Entra-federated Keycloak groups bound to viewer / developer / owner | `GET` / `POST` / `DELETE …/access` | read: member; write: **owner** |
| **Branch** access: one of five postures — `panacea-sso`, `kpmg-sso` (safe default), `password`, `bypass-token`, `none` | `GET` / `PUT …/branches/{branch}/access` | read: member; write: developer |

Two controls, not one. Presenting a merged *"who can see this branch"* control would invent a
capability (survey §4). Group names are never joined with `","` — Entra allows commas in display
names and the platform's header parser already splits on them (survey §4).

### 2.5 Provenance — the release carries its thread

| What | Source |
|---|---|
| Each release row names its pinned brief revision, or says *"no brief is recorded for this release"* | release link, written by the Ship handler in the same operation as `POST …/deploy` (contract §2.2, P7) |
| The shipped manifest renders the **pinned** revision's text, never the head's; says when the brief has moved on | `GET /api/v1/discover/refs?key=…` (contract §3.1, §4.5) |
| The design link and merge link, beside the pin, with disagreement shown as disagreement | contract §3.4 |
| Criteria listed with *"no evidence recorded yet"* until phase 3's evaluator exists | contract §4.5, P12 |
| **Which sessions produced it** | **no source exists** — decision D3 (§0) |

### 2.6 A name you choose — the URL prefix (owner-only)

**Read from Panacea source for this brief; the survey did not cover it.**
`handlers/branch_url_prefix.go`: a branch can claim a **globally unique**, human-readable host —
`<prefix>.<domain>`, or `<prefix>.<team>.<domain>` — instead of the derived host. The prefix is an
RFC 1123 label (1–63 characters, lowercase, digits, hyphens), platform-reserved labels are refused,
and the claim is first-come (`INSERT … ON CONFLICT DO NOTHING`). Guard: **owner**
(`routes.go:1313–1314`).

This is the only part of the brief's *"branch environments with arbitrary names"* the platform
actually supports. It names the **URL**, not the namespace — the namespace is still the truncated
branch name (R2). Whether it is in scope for phase 7 is part of decision D2.

## 3. Explicitly out of scope

| Out | Why |
|---|---|
| Any new `panacea-api` endpoint | The read and act paths exist. The metadata ask (a client-set annotation on a deploy) is raised in parallel by the orchestrator and **must not block** phase 7 (contract §7.3) |
| Desktop OIDC / PKCE login | No public or loopback Keycloak client exists in realm `panacea`; creating one is a change to the Panacea platform, not ours (survey §5.3) |
| Per-branch Entra group binding | The API does not offer it (§2.4). Offering it would mean building it on the platform — assertion B1 |
| Naming the **environment** freely | The platform derives it. We never re-implement `sanitizeBranchEnv` / `mapBranchToEnvWithProd` — that would be a fourth copy to drift (survey §3.2) |
| Minting tokens from the app | A token cannot bootstrap a token (`authedUnscoped` rejects scoped tokens) |
| The ten per-deploy scan-mode overrides on Ship (`secretScanMode` … `schemathesisMode`) | Turning `enforce` into `off` from a desktop is a security-posture change, and the Panacea owner's rule is *off means skip, never fabricate*. Ship sends none; the platform's defaults apply. The lens **shows** each scan's mode |
| Promote, archive, pause, delete branch, set-primary | Routes exist; not in the phase brief. Rollback and redeploy cover the brief's "rollback" |
| Evaluating criteria against the shipped commit | Phase 3's evaluator (contract §2.3). Phase 7 renders `open` and *"no evidence recorded yet"* |
| Kata, capacity, customer-hosted tier, direct Azure calls | Phases 6, 8, 9 and the later hosting tier |

## 4. Seams

### 4.1 Against phase 2 (the cockpit, live in this tree)

Phase 2 is being edited by another agent right now. Everything here was read, not changed.

| Seam | What phase 7 needs | Status |
|---|---|---|
| **The deploy lens slot** | `AppShell.tsx` `case "deploy"` renders a `LensPlaceholder` | **Observed in the working tree:** it now says `phase={7}` (the survey found `{5}`). The file is modified and uncommitted, so this is not yet on the branch |
| **Failure vocabulary** — `desktop/src/api/problems.ts` | `ProblemKind` has three members and the comment on `backend-down` says it means **Docker**. Panacea being unreachable, a rejected token (401), a pending team approval (403) and an owner-only 403 are four new facts. Mapping any of them to `backend-down` would make the cockpit say "Docker" about Panacea | **Recommended, not decided:** the control plane writes each sentence and returns it as `server-said`, which renders verbatim. That needs **no** phase 2 change. The alternative — a fourth `ProblemKind` — breaks every call site by design (TS2366, per the file's own comment) and is a phase 2 contract amendment |
| **One poller, one heartbeat** | Deploy data joins the phase 2 poller's single request cycle; never a `useEffect` fetch per row (contract P8). SSE opens only for the one drilled-in environment | I did **not** read `poller.ts`. The cadence and whether it can take a second backend (Panacea, via the control plane) are unverified |
| **Persisted desktop state** | Deploy data is server truth and is **not persisted**, so phase 7 bumps no persist version (contract §6.5, A5) | Recommendation |
| **Control-plane routes** | Every phase 7 route goes through `routes.Handle` in `control-plane/internal/api/api.go`; `/api/v1/` is already a `StripPrefix` alias of `/v1/` | From the survey §7, not re-read |
| **Control-plane authentication** | The control plane will hold a `pnc_` token. Loopback bind, Host guard and Origin allowlist stop a browser; they do **not** stop another local user's process on `127.0.0.1:8730` — which could then read and, once Ship exists, deploy as the user | **Unowned.** Contract §6.6 and open question 6: `client.ts` says auth arrives in phase 8, the phase 4 plan says phase 9, `PROGRAM.md` names neither. **It gates phase 7's first task, reads included** |
| **Lens is a peer, not a stage** | `AGENTIC-UX.md` principle 12: no numbering, no arrows, no completion states across lenses | The prototype carries no step numbers |

### 4.2 Against the provenance contract draft

The contract is a draft awaiting your review. Phase 7 adopts it as written; where it does not
fit the platform, that is stated here rather than patched.

| Contract says | Phase 7 does | Fit |
|---|---|---|
| Release link `(projectName, pipelineRun) → brief key + digest (+ design key)`, written **in the same operation** as `POST …/deploy` (§2.2, P7) | The Ship handler writes the link on the 202, which returns the run name synchronously | Fits Ship |
| Rollback, promote and redeploy inherit the re-run release's pin (§3.4) | Same | **Does not fit rollback's key.** Re-read for this brief: `Rollback` (`projects.go:1629`) returns **200** with a `Deployment` in an `APIResponse` envelope, and `RollbackWarmth` describes a **scale-up of an existing immutable Deployment plus a Service re-point** (warm) or an image patch (cold). **No PipelineRun is created** — *inferred* from the handler and the field comment; I did not read `RollbackDeployment` itself. So a rollback has no `pipelineRun` to key the link on. The contract needs a second key shape for rollback — e.g. `(projectName, environment, deploymentId-rolled-back-to, time)`. Phase 4 owns the ledger schema; phase 7 raises it |
| Never pin by observation (P2) | Releases the poller sees but did not ship — auto-deploy on push, the Panacea portal — show *"no brief is recorded for this release"* | Fits. **Auto-deploy therefore produces unpinned releases by construction.** The prototype shows this at the auto-deploy control, not only on the row |
| Six provenance states: current, superseded, none, missing, resolving, unavailable (§4.1) | Each is drawn in the prototype | Fits |
| Manifest renders the pinned revision's text; criteria *"no evidence recorded yet"* (§4.5, P12) | Same; no denominator, no "2 of 4 met" | Fits |
| Design link and merge link shown beside the pin; disagreement shown (§3.4) | Same | Fits — depends on phases 5 and 3 existing |
| Ledger loss makes a pinned release read as unpinned, unless the metadata ask is granted (§3.4) | The required wording is true in both cases | Fits; a known limit, stated in the lens's help text, not hidden |
| — *(nothing)* | The phase brief asks for **"which sessions produced it"** | **Gap.** No link kind names sessions. `contextRef` records what a session *inherited*, not what it *contributed* to a commit. Decision D3 |
| Ledger and brief store in the control plane's state directory (§2.1) | — | **Per person, not per project** while the control plane is a local process. A teammate's release has no link in my ledger, and a teammate's brief key would not resolve in my store. Decision D1. *Inferred* from §2.1, §6.6 and `PROGRAM.md`; the contract does not discuss more than one user |

**Ship ships what Panacea can fetch, not what is on disk.** `TriggerDeployOptions` carries a
`branch` and a `commit` and Panacea's pipeline clones from its git remote. Commits a session has
made in its worktree but not pushed are **not** in the build. The lens should say so beside Ship
(*"2 local commits are not pushed and will not ship"*), which needs phase 3's git API to report
ahead/behind for the branch. *Inferred* from the request shape; the Tekton clone task was not read.

### 4.3 Against phase 3 (git, worktrees, convergence) and phase 4 (Discover)

- **Merge link home** — ledger or git trailer — is phase 3's call (contract §2.2). Phase 7 reads
  whichever it is.
- **Criterion evaluator** takes `(brief key, commit)` and phase 7 passes `Deployment.Commit`
  (contract §2.3). Until it exists, every criterion is `open`.
- **Session attribution** (D3, option b) would need phase 3 to stamp a session id on every agent
  commit. Phase 3 has no plan yet; the obligation would be added to contract §8.3.
- **Brief resolver** — `GET /api/v1/discover/refs` batch — is phase 4's (contract §3.1). Phase 7
  sends every visible release's key in one request per heartbeat edge.
- **One click to the text** — Discover must open a past revision read-only (contract §4.1). Phase 7
  links to it and does not render its own copy (P4).

## 5. Risks, and who decides each

### 5.1 Decide before a plan is written

| # | Risk | Consequence if ignored | Who decides |
|---|---|---|---|
| **R1** | **Control-plane auth is unowned** and the control plane is about to hold a Panacea token (§4.1) | Any local process on the machine reads the user's Panacea projects, and after Ship exists, deploys as them | **You** — contract open question 6. Gates the first task, reads included |
| **R2** | **Branch names collide at 20 characters.** `sanitizeBranchEnv` lowercases, turns `/` and `_` into `-`, truncates to 20, trims a trailing `-`. `feat/phase-7-deploy-connector-a` and `…-b` both become `feat-phase-7-deploy` | The second Ship silently overwrites the first branch's environment. No error. This is the *silent-override failure class* Panacea has already paid for repeatedly. Detecting it **before** Ship needs either a fourth copy of the mapping (survey §3.2 says do not) or a platform change. Detecting it **after** is possible: the environment's newest release names a different `Branch` | **You** — D2 |
| **R3** | **"EntraID groups per branch" does not exist** (§2.4) | The brief promises a control the API does not have. Building the UI as briefed means inventing a capability | **You** — assertion B1; overrule it to ask the platform |
| **R4** | **"Which sessions produced it" has no source** (§4.2) | Either the element is dropped, or it is drawn from a fact that does not say what the label claims — `AGENTIC-UX.md` §2 property 7 forbids the second | **You** — D3 |
| **R0** | **Provenance is per person** while the ledger and brief store are local (D1) | On a team project, most releases on screen are someone else's and read *"no brief is recorded"*. The spine claim holds for one person at a time | **You** — D1 |
| **R5** | **Provenance cannot round-trip through Panacea.** `TriggerDeployOptions` has no client-set metadata; `DeployedBy` and `Trigger` are `json:"-"` | The link lives only in our state directory. Lose it and pinned releases read as unpinned; the Panacea portal never shows the brief | **Orchestrator** — whether to raise the metadata ask now (contract §7.3). **You** — contract open question 2, for releases shipped outside the cockpit |

### 5.2 Designed for, or someone else's

| # | Risk | Handling | Who decides |
|---|---|---|---|
| R6 | **Build green, site dead.** `Status: Succeeded` with `LiveReadyReplicas: 0` is normal | Two facts, two lines, always. Unmeasured says nothing rather than implying health | Settled by this design |
| R7 | **Owner-only controls** — auto-deploy, project access writes, URL prefix | Drawn disabled for non-owners, with the reason; never discovered as a live 403 | Settled by this design |
| R8 | **Rollback to a pruned image.** Re-read for this brief: `Rollback` refuses with **409**, *"cannot roll back: image … no longer exists in the registry (it may have been pruned) — redeploy from source instead"* | Rendered verbatim, with Redeploy offered. Drawn in the prototype | Settled by this design |
| R9 | **No native OIDC client** | Paste-a-token onboarding. Desktop login waits for a public Keycloak client in realm `panacea` | **Panacea platform owner** |
| R10 | **Corporate TLS interception** — `panacea-mcp` needed `PANACEA_TLS_INSECURE=1` | Root-cause fix is loading the corporate root CA; an insecure flag is a workaround and, if kept, opt-in and loudly logged | **Security owner** |
| R11 | **Base URL unresolved** — `api.dev.panacea.kpmg` vs `dev.panacea.kpmg/api` | Configuration, not code. One unauthenticated `GET /api/v1/version` settles it | Whoever writes the plan, with one network call |
| R12 | **Offboarding latency on tokens** depends on `LiveGroupResolver` being wired; without it a token keeps its frozen group snapshot | Nothing phase 7 can change | **Panacea platform owner** — verify the live wiring |

## 6. What I could not verify

- **No network call was made.** No endpoint probed, no token used, no response body observed.
  Every shape is read from Go source or from the survey.
- **Read from Panacea source for this brief** (the rest is the survey's reading, carried
  through): `TriggerDeployOptions` and `PipelineRunStatus` (`models.go:2342–2397`);
  `SecurityStatus`, `ScanResult`, `BlockingItem` (`models/security.go:117–242`); `Rollback`
  (`handlers/projects.go:1629–1720`); `RollbackWarmth` / `ColdStartSeconds` (`models.go:695–708`);
  `handlers/branch_url_prefix.go` (header and validation).
- **The survey says eleven scan-mode overrides; the struct has ten.** Minor, but a plan that
  copies "eleven" will look for a field that is not there.
- **That rollback creates no PipelineRun** is inferred from the handler's response shape and the
  `RollbackWarmth` comment. I did not read `K8s.RollbackDeployment`.
- **Whether a rollback appears as a new row in `GET …/deployments`**, and what its `Trigger` is.
  The prototype draws it as a new row marked *rollback*; if the platform does not record one, the
  lens must derive it from our ledger alone.
- **Whether rollback warmth can be known before pressing.** The field is set only on the object
  `Rollback` *returns*. The survey says the lens can say "instant" vs "~20s" *before* the user
  commits; the source says the hint arrives with the response. The prototype shows it **after**.
- **`GET …/environments` does name a branch per environment** — re-read for this brief
  (`handlers/projects.go`, the response map carries `name`, `branch`, `displayName`, `primary`,
  `subdomain`, `status`, `lastDeploy`, `replicas`). **Not verified:** which branch it reports when
  two branches collide on one namespace — presumably the last to deploy. The prototype's collision
  state relies on it reporting the other branch.
- **Rollback also refuses with 400** *"deployment has no image tag — cannot rollback"* for a
  release with no recorded image. Read in the handler; drawn in the prototype.
- **`Deployment.provisioning[]`** exists (*"app deployed but database provisioning failed"*). It is
  a third fact beside build and serving that the prototype does **not** draw yet — the next state to
  add if you want it.
- **Whether two branches sharing a namespace also share a URL-prefix annotation**
  (`panacea.io/url-host` is a namespace annotation). Inferred likely; not read.
- **`poller.ts`** — cadence, and whether it can carry a second backend.
- **The phase 2 `AppShell.tsx` change to `phase={7}`** is in the working tree, uncommitted.

## 7. How the prototype maps to this brief

Open `prototype/phase-7-deploy.html` by double-click. The strip at the top is review chrome, not
product: **State** switches between the seventeen states below, **Role** switches developer and
owner, **Show sources** prints under each element the API field or ledger record that justifies it.
Click a release row for its manifest.

| Group | States drawn | What to look at |
|---|---|---|
| Connection | not connected · awaiting team approval (403, verbatim) · token rejected (401) · Panacea unreachable (last reading kept, dimmed, marked stale; Ship and rollback off) | Onboarding says where the token comes from; the address is a field, not a default, because two sources disagree |
| This branch | never deployed · building · live · built green with the site down · build failed · scan blocking · scans incomplete · environment taken by another branch | Build, serving and scans are **three separate facts** in every state. While building, only finished steps and the current step are listed — Panacea does not say what remains, so there is no bar and no count |
| Rollback | warm · cold · refused because the image was pruned (409, verbatim) | How fast a rollback is appears **after** it starts, because that is when Panacea says. Rollback keeps the old release's pin |
| Provenance | brief store unavailable · branch with no brief | In **live**, the release rows show `current`, `superseded`, `none`, `missing` at once; every state change shows `resolving` for a moment — a fixed-width placeholder, no spinner, no width change |

**Design-system rules the prototype holds, and how:**

- **Colour** — every literal colour is a token in `:root`; a grep for hex or `rgba(` outside it
  returns nothing. Values are `prototype/app.html`'s, plus tints, rings and shadow colours it lacked.
- **Elevation** — cards, banners, the releases table and toasts use a 1px ring plus a layered
  shadow (`--e-card`, `--e-raised`, `--e-overlay`). Structural dividers are hairlines.
- **Radius** — `DESIGN-SYSTEM.md`'s scale: chip 6, control 8, card 10, window 14. `app.html` uses
  7 and 9; this file follows the design system, which is binding.
- **One heartbeat** — every live dot and the build hairline share one CSS clock: each render sets one
  negative `animation-delay` from a single time origin, so elements inserted at different moments
  still breathe in phase. One `setInterval` updates live numbers in place and never re-renders.
- **Motion** — `transform` and `opacity` only; no `transition: all`; a changed fact flashes once
  and decays over 600ms; `prefers-reduced-motion` stops all of it.
- **Numbers** — `tabular-nums` on the whole document. No counts, percentages or health scores
  (`AGENTIC-UX.md` principles 4 and 10).

**Not verified: how it looks.** The file was checked mechanically — every state, both roles and every
release selection render without an undefined value in a stub-DOM run, and the colour, transition and
network greps are clean. It has **not** been seen in a browser by anyone; headless screenshots are
blocked on the corporate network. Visual acceptance is yours.
