# Phase 7 — Deploy connector: adversarial plan review

**Reviewer:** adversarial plan reviewer (read-only except this file), 2026-09-24.
**Plan reviewed:** `docs/superpowers/plans/2026-09-23-phase-7-deploy.md` at `897677b`, with its
20 parts in `2026-09-23-phase-7-deploy-parts/`.
**Method:** one part at a time, in line ranges. Every Panacea citation checked below was opened in
`~/Projects/Panacea/services/panacea-api` (read-only). No Panacea environment was called. Nothing
was compiled or run; every Go and TypeScript block in the plan is still uncompiled.

Line references are `file:line`. `plan` = the main plan file; `tNN` = `…-parts/task-NN.md`.
Panacea paths are relative to `services/panacea-api/`, read at `b02424691`.

**Verdict: Needs fixes.** 3 Critical, 9 Important, 30 Minor. None of the three Critical defects
would fail a test the plan writes. All three show up only in the Tauri window or against live
Panacea.

---

## Coverage

**Owner decisions.**
- OD-1: covered. UI branches stay unmerged, and every UI task says "visual acceptance pending".
- OD-3: covered. Releases and pins live in the Hub (plan:120).
- OD-4: covered in intent (`githost.ParseRemote`, plan:121). Azure DevOps is broken in practice by
  Important I6.
- OD-6: covered. There is no sign-in of its own; `hubclient` and `credstore` come from Hub C1/C2.
- OD-12: covered. `LIVE-CHECK.md` batches every live item (t20:380-410). Task 02 breaks this, see
  I9.
- OD-13: covered. `deploy.Deployer` plus a conformance suite, and no Panacea source is copied. The
  swap surface is larger than S7-N1 says (Minor M28).
- OD-2, 5, 7-11: not relevant to phase 7, and correctly not claimed.

**Scope brief (`scope-and-risk-phase-7-deploy.md`).**
- D1: answered by OD-3.
- D2 (a), plus the URL prefix: tasks 08, 11, 18.
- D3 (b), trailers: task 19.
- B1 (access for a person, not a group): tasks 04, 11, 18.
- B2 (no scan override): t04:20-30.
- B3 (5 s hold): tasks 16 and 18. The plan's text is looser than the parts (M24).
- B4 (absolute times): t08:154-155.
- §2.1 connect: task 05. §2.2 see: tasks 03, 07, 08, 15 and 17. §2.3 act: tasks 09 and 10. §2.4
  govern: tasks 11 and 18. §2.5 provenance: tasks 15, 17 and 19. §2.6 URL prefix: tasks 11 and 18.
- §3 out of scope: honoured. There is no new Panacea endpoint, no second copy of
  `sanitizeBranchEnv`, no scan overrides and no token minting.
- The brief's "redeploy" becomes "Rebuild" (7P-3), and plan:693 records it as a question for the
  owner (Q-P7-1). That is sound.

**Prototype (`prototype/phase-7-deploy.html`, the review select at `:337-376`).** All 17 states
have a home:
- the connection states (connect, approval, rejected, offline): task 14;
- never deployed, building, live, dead, failed, scans blocking, incomplete: task 15;
- collision: tasks 08 and 15;
- rolled back warm and cold: task 16;
- rollback refused because the image was pruned: tasks 10 and 16;
- the brief resolver unavailable: task 15's `unavailable` pin state;
- no brief: plan Q2, t15:93-94.

Both prototype roles, developer and owner, are drawn through `ownerControls` (7P-9). The prototype
has no viewer role, so neither does the plan.

**Orchestrator rulings.**
- R4 (`upstream-down`): t13:250-275.
- R5 (rollback union): t10:57-66. It matches Hub contract §7.9:993-997 and 1021.
- R6 (one selector): t13:303-306.
- R7 (the metadata ask, drafted): t20:412-466.
- R2 (`writeAtomic` lift): t06:190-194.
- R3 (no persist version): plan:321.

**Hub contract.**
- Reserve, confirm and abandon, idempotency, `not_reserver` and expiry: task 09.
- The deployment join, and its uniqueness: t07:129-134.
- The relay rule: t05:202-204.
- The credential store row `PanaceaApp/panacea/<host>`: t05:39-44 and Hub contract:1175.
- The cursor mechanism: HA7-4 and t07:148-166.
- Gaps: the release list is capped at 200 rows (M11). The Hub change feed's `release.*` events never
  reach the watcher (M12). The ADO repo-key spelling is Important I6.

**Panacea citations checked, all matching at `b02424691`:**
- all 26 `routes.go` lines of plan §4, plus 3050;
- the DTO tags against `models.go` for `Deployment` (665-735), `ScanGate`, `ProvisioningStatus` and
  `PipelineRunStatus` (2386-2408);
- `security.go` 117-242, and `NamespaceEvent` at `clients/pod_introspection.go:95-104`;
- the envelopes (`pod_introspection.go:139,199`, `diagnose.go:157`, security-status,
  `ListEnvironments`, `GetPipelineRunStatus`);
- the TriggerDeploy 202 and its lenient decode, the Cancel bodies, and the Rollback body with
  `ID == deployID` (`kubernetes.go:5982,6030`);
- the access Add 201, the posture GET and PUT shapes and the 409 lock, `canonicalAccessMode`, and
  the SSE frames.

Checking them turned up the facts behind Critical C2 and C3 and Important I2, I4 and I5.

---

## Strengths

1. **Provenance is taken at the right place.** The Hub fixes the pin at reserve (G1). Phase 7
   sends only the branch (t09:169-172), and a reserve failure means Panacea is never called
   (t09:224). The run name is the join key, and it is known before the row exists (plan:96-99,
   verified at `kubernetes.go:5743`). The plan never pins by observation. It says the timeout case
   is ambiguous (t09:178-183) rather than guessing a pin.
2. **R5's premise was actually verified.** 7P-4 reads `RollbackDeployment` and finds no
   PipelineRun and `ID: deployID` (checked: `kubernetes.go:5982`, `:6030`). So a rollback confirms
   with `{}` and never synthesises a run. HA7-2 turns the Hub's loose sentence into a test.
3. **The `pnc_` handling holds up.** 7P-1's case against the Entra bearer is sourced four ways.
   The token enters through a write-only route, is stored only after `/me` answers 200, and is
   never sent to an address that failed `Version` (t05:225-226). It never enters the zustand
   store or a `useState` (t14:69-73, t14:82). A redacting `String()` and log-capture tests guard
   the logs. There is no skip-verify flag.
4. **The three facts are never merged.** Build, serving (the nil pointer against 0) and scans
   stay separate from the DTO (t03:87) through the conformance rows (t01:545-546), the view
   (t08:50-64) and the card (t15:73-87). A test pins each one.
5. **The design system is honoured in every UI task.** There is one heartbeat
   (`beatDelayStyle` only through `StatusDot`, checked at `StatusDot.tsx:2`). The log pane
   flushes at about 10 Hz through the shared `FLUSH_INTERVAL_MS` (`attachRegistry.ts:17`), never
   scroll-jumps, and keeps at most 5,000 lines. There are no spinners, `tabular-nums` is used,
   elevation and radius tokens exist (`tokens.css:108-121`), the drawer moves by transform and
   opacity only, and every screen is designed for every state.
6. **Test-first, with named mutants.** Every part opens with failing tests. Almost every row names
   the mutant it kills, and the conformance suite is the swap proof for OD-13.
7. **It is honest about what is unverified.** Plan §12 and every *does NOT claim* block list the
   gaps. `LIVE-CHECK.md` turns each one into a step with a *Seen* column.
8. **Polling is disciplined.** One watcher does all the polling. `deployments` is read only on
   change or once a minute (7P-5). The cursor rides on the existing `GET /v1/sessions`, so the
   desktop adds no interval (verified: `poller.ts:84` and `heartbeat.ts:34` are the only
   `setInterval` sites).

**Task sizing and order** are reasonable: 20 tasks of 1.5-3 h, and 7a (reads) is separated from 7b
(writes, gated on phase 3). The exception is task 03. Its 3 h also has to cover 16 DTOs, around 20
recorded bodies and `panaceatest.Server`, a stateful fake `panacea-api` that implements `Seeder`
and every read route. That is closer to 5 h, so split the server out, or re-estimate.

---

## Issues

### Critical

**C1: The control plane's CORS preflight blocks `PUT` and the `Idempotency-Key` header, so in the
Tauri window Connect, Bind, every governance write, Ship, Rollback and Rebuild all fail.**
- **Where.** `control-plane/internal/httpx/origin.go:169-170` sets `corsMethods = "GET, POST,
  PATCH, DELETE, OPTIONS"` and `corsHeaders = "Content-Type"`. Both are served at `:232-233`. Hub
  task 37 only adds `Authorization` (`hub/.../phase-h-hub-parts/task-37.md:144-145`).
- **What phase 7 sends:**
  - `PUT /v1/deploy/connection` (t05:149)
  - `PUT …/deploy/binding` (t06:152)
  - `PUT` for auto-deploy, posture and url-prefix (t11:37, 45, 71)
  - `Idempotency-Key` on ship, rollback and rebuild (t09:198, t10:103, t10:116, t13:322)
- **Wrong.** No part touches `corsMethods` or `corsHeaders`. t07:17 changes `origin.go` only for
  `Expose-Headers`. The webview is cross-origin to `127.0.0.1:8730`, so each of these requests
  is preflighted and refused.
- **Why it matters.** Every Go test calls handlers directly, and every vitest test mocks `fetch`,
  so the whole plan is green while the lens cannot connect or ship. It is the silent-override
  class, and the owner's session (OD-12) is the first place anyone would see it.
- **Fix.** In task 05 (the first `PUT`), add `PUT` to `corsMethods` and `Idempotency-Key` to
  `corsHeaders`. Declare it as an amendment to phase 2 and Hub C3. Add an `origin_test.go`
  preflight row for each: an `OPTIONS` with `Access-Control-Request-Method: PUT` and one with
  `Access-Control-Request-Headers: idempotency-key`, both asserting the allow lists. The mutant is
  "constant unchanged".

**C2: `environments[].branch` is Panacea's environment key, not the git branch, so every branch
with `/`, `_`, capitals or more than 20 characters is refused as "never deployed" and has no
environment card.**
- **Evidence.** `ListProjectEnvironments` builds branch rows as `{Env: env, Branch: env}` from the
  namespace suffix (`clients/kubernetes.go:5163-5240`). The handler says so: *"For non-base envs
  Env==Branch already"* (`handlers/projects.go:2109-2111`, the row at `:2116-2118`). So
  `feat/evidence-upload` is served as `branch: "feat-evidence-upload"`.
- **Wrong:**
  - Ship step 4 matches `row.branch == Branch.Name` exactly (t09:167-168). The result is `409
    deploy_environment_unknown`, with the false sentence *"This branch has never been deployed"*
    (plan:579-583).
  - The card's `environments.find(e => e.branch === branchName)` (t15:67-68) finds nothing.
  - The collision banner (`env.branch !== branchName`, t15:116) fires falsely.
  - The t08:42-43 example itself shows a row with `branch: "feat/evidence-upload"`, which Panacea
    never returns.
- **Why it matters.** Phase 3 lanes and ordinary feature branches almost all contain `/`. Ship,
  the lens's main act, would work only for trunk-like names. Fakes and recorded bodies use
  matching names, so no test catches it.
- **Fix, with no fourth `sanitizeBranchEnv`.** `GET P/branches/{branch}/access` (`routes.go:1430`,
  member-readable, already row 15 of plan §4) returns `data.env` from `branchEnv` →
  `mapBranchToEnvWithProd`. That is Panacea's own mapping, for **any** branch, with no existence
  check (`handlers/branch_security.go:29-37`; `project_security.go:646`, `:668-677`). Resolve
  branch → environment through it, then join to rows with `SameEnvironment`. This also shows that
  HA7-1's premise (*"no route reports it before the deploy"*, plan:563-564) is false: HA7-1 is no
  longer needed for a first Ship.
- **Tests:** a `feat/x` branch whose row is `feat-x`, and a branch longer than 20 characters.

**C3: Scans are read without the environment, so every non-production release shows
production's scan results.**
- **Where.** `Scans(ctx, p, deploymentID)` (t01:70) sends only `?deployId=` (t03:204, t12:60,
  t20:271).
- **Evidence.** `resolveEnv` defaults a missing `env` to `"production"`
  (`handlers/security_status.go:192-201`). `collectScans(ctx, name, env, deployID)` then reads
  that environment's namespace (`:231`), and *"env-keyed scanners still surface their
  latest-per-env result"* (`:211-213`). Panacea's own portal sends
  `env=<deploy.environment>&deployId=<id>` (`backstage/.../SecurityScanReview.tsx:429-440`).
- **Why it matters.** The lens's third fact, and the drawer's scan list, would show another
  environment's PASS or FAIL, or `NOT_RUN` rows, as this release's verdict. In a regulated product
  that is a false security statement, and nothing flags it.
- **Fix.** Make the domain method `Scans(ctx, p, env, deploymentID)`, and send `env` when it is
  non-empty, as the portal does. The watcher passes the newest deployment's `Environment`. Add
  `environment` to the drill route. `panaceatest.Server` should answer a request without `env`
  with production's report. The test: a branch deployment's scans read with `env=feat-x`. The
  mutant: dropping `env`.

### Important

**I1: Five acceptance greps cannot give the output they promise.**
- **The five:**
  - t01:581-582 uses `-i`, so `"Pending"` and `"Succeeded"` match the constants `"pending"` and
    `"succeeded"` (t01:145-147). The Panacea-naming comments (t01:24-26, :82, :402) match too.
  - t03:311-312 greps `// mirrors`, but the comments read `// xDTO mirrors …`, so the count is 0.
    There are also 16 DTOs, not 17.
  - t04:196-197: `Mode"` matches `json:"accessMode"` (t04:68, :78, :88). The shipRequestDTO
    comment spells `scan-mode`, so it does not match.
  - t05:260-261: `pnc_` also appears in the `TokenSource` comment (t02:24).
  - t07:203-204: the grep returns 3 lines, because `heartbeat.ts:29` holds
    `ReturnType<typeof setInterval>`.
- **Why it matters.** This is the first defect an implementer meets, in task 01's first hour. They
  either delete useful comments to make the grep pass, or learn to ignore acceptance lines.
- **Fix.** Make each grep case-sensitive and anchored to the thing it guards (for example
  `grep -nE '"(Succeeded|Pending|Failed)"'` without `-i`), or state the real expected lines. Run
  each grep once against the part's own code block before committing.

**I2: Panacea's budget refusal reads as "Panacea is down".** `TriggerDeploy` runs
`enforceTeamBudget` (`handlers/projects.go:2962`), which answers **402** *"new deploys are paused:
<reason>"* (`:2856`). The status table maps "any other" to `KindUnavailable` with an empty
message (t02:117). That becomes 503 `upstream_unavailable`, then `upstream-down`, and the lens
shows Panacea offline and switches acts off (R4). A `KindUnavailable` with status 402 matches none
of task 09's abandon branches (t09:174-183), so the reservation is never abandoned: it expires
after 15 minutes, and Panacea's reason is lost. Fix: map 402 to `KindConflict` (or a new
`KindRefused`) with the sentence. Add `ship-402-budget.json` and a test.

**I3: The rollback's "what is serving now" rule is wrong in both directions.**
- **Where.** Rule 6 (t08:143-146) and 7P-4 (plan:227-229) say the current release is the newest
  deployment, unless a confirmed rollback's `confirmedAt` is later than that deployment's
  `createdAt`.
- **Wrong:**
  - `createdAt` is PipelineRun creation (`kubernetes.go:5680`, `:5754`), not when the build went
    live.
  - The newest deployment is current even when it **failed** or is still **running**.
- **What the user sees:**
  - A failed newest build is marked `isCurrent` while the old one serves. That also hides *Roll
    back* on the release that really is serving (t16:95).
  - Ship at 10:00, roll back at 10:05, and the build goes live at 10:08: the lens says the
    rollback is serving.
  - `TestRollbackBeforeNewerDeploy` (t08:172) enshrines the error.
- **Fix.** The current release is the newest **succeeded** deployment by completion time
  (`createdAt` + `duration`, `models.go` `Duration`), or the newest confirmed rollback confirmed
  after it. Running and failed builds are never current. Add tests for both cases.

**I4: `Deployment.branch` is the sanitised label. This settles t08:197-202's "unverified" point,
and the plan builds on the wrong answer.**
- **Evidence.** The label is `sanitizeLabelValue(branch)` (`clients/pipeline.go:609`), which turns
  `/` and `_` into `-` (`:2351-2364`), and it is read at `kubernetes.go:5750`. There is no
  annotation holding the real branch.
- **Consequences:**
  - `branchesSeen` compares sanitised names, so t08:151-153's claim of exact real-name comparison
    is false.
  - Rebuild's fallback sends the sanitised name as a git branch, and as the Hub `branchRef`
    (t10:82-83).
  - `environmentForBranch` compares `branch == prodBranch` (`pipeline.go:196-206`), so rebuilding
    a production branch named `release/x` lands in a branch environment.
- **Fix.** Record the fact in plan §12 as settled. Drop the `source.Branch` fallback (M23). Word
  the collision banner and `branchesSeen` as label values.

**I5: Scoped `pnc_` keys are not handled anywhere.**
- **What a scoped key is.** Panacea keys can be confined to **one** project at role `read`
  (Viewer) or `deploy` (Developer), and never above Developer (`middleware/api_key.go:268-329`,
  `:301-303`; `models/models.go:550-559`).
- **What goes wrong with one:**
  - `GET /projects` returns one project, so other local projects read *"No Panacea project you can
    see deploys this repository"*.
  - The owner probe (7P-9) always gets 403, so a real owner is told *"Only a project owner can
    change this"* (t18:111).
  - A `read` key's Ship is refused.
- **Fix.** The connect panel says which key to mint (unscoped, or the owner's choice). The
  owner-only and no-match sentences say *"your key or your role"*. Add a recorded scoped-key `/me`
  and `/projects` pair. Add an owner question: unscoped key, or a `deploy`-scoped one?

**I6: Azure DevOps repositories can never Ship, because the plan wrongly says phase 7 is not
affected by the repo-key spelling split.**
- **The claim.** Plan:644-648: *"Phase 7 only ever compares keys it computed itself with `githost`
  on both sides."*
- **Why it is false.** t06:106 (step 3) sends the `githost` key to the Hub's
  `ProjectForRepo(RepoKey)`. The Hub compares byte for byte against keys matching
  `^[a-z0-9-]{1,50}\.visualstudio\.com/…` (Hub contract:700-715). If `githost` spells
  `azure-devops/<org>/…` (plan:646), every ADO repository gets `HubProjectID ""`. Ship is then off
  (`no_hub_project`, t09:158), which breaks OD-4.
- **Fix.** Raise it as a blocking cross-phase ask (HA7-6, to phase 3 and the Hub). Add a
  `binding` test against the Hub fake using the Hub's ADO spelling. Until it is settled,
  readiness names the cause.

**I7: Ship readiness never refreshes after a push.**
- **Where.** Readiness reloads only on the deploy cursor, a branch selection or the lens opening
  (t16:126, :131-133).
- **Wrong.** `ahead` and `behind` are phase 3's local facts. They change on a push without any
  deploy-cursor bump. The user presses phase 3's **Push** beside the disabled Ship (t16:83), and
  Ship still says *"2 commits are not pushed"* until they re-select the branch.
- **Fix.** Add a fourth edge: the branch's convergence (tip, ahead, behind) changing in the store
  on the existing tick, plus Push completing. Still no timer. Test: push, then Ship turns on with
  no reselect.

**I8: The drawer re-joins environments by exact name, so production rows never get *Why it is
down* or *Follow readiness*.**
- **Where.** t17:121-122: `view.environments.find(e => e.name === row.environment)`.
- **Wrong.** Production deployments carry the label `prod` (`environmentForBranch`,
  `pipeline.go:196-206` via `:237`), but the row is named `production` (`projects.go:2116-2118`).
  Task 08 joins with `SameEnvironment` (t08:129-132), and the desktop then undoes that join.
- **Fix.** The view puts the matched row's name on each `ReleaseRow`, and the desktop never
  re-joins. Add a test with a `prod` row.

**I9: Task 02's acceptance needs a live network call, which breaks OD-12 and the no-Panacea rule
for agents.**
- **Where.** t02:145-151 and :160-161 run `versioncheck` against `https://dev.panacea.kpmg` as the
  implementer's acceptance.
- **Why it matters.** A subagent running this plan is told never to call a real Panacea, and
  OD-12 batches live items. So the task cannot be accepted as written.
- **Fix.** Delete the step from task 02. It already exists as `LIVE-CHECK.md` step 1 (t20:393).
  Keep `cmd/versioncheck` as a file.

### Minor

| # | Where | What is wrong | Why it matters, and the fix |
|---|---|---|---|
| M1 | t01:363-367, :486 | The domain's `PostureMode` constants are Panacea's wire words, against t01:8's rule. The claim "exactly as `canonicalAccessMode`" is also loose: Panacea maps an unknown mode to `none` when `ssoEnforced == "false"` (`access_mode.go:75-77`) | Say the domain adopts this vocabulary on purpose, and drop "exactly". Harmless, because Panacea's GET already canonicalises |
| M2 | t02:74-78 | `BuildHTTPClient` does not say to clone `http.DefaultTransport`. A fresh `&http.Transport{}` has `Proxy: nil`, so it ignores `HTTPS_PROXY`, and it loses HTTP/2 and the idle-connection defaults | A likely first failure on a corporate network. Clone `DefaultTransport`, keep `ProxyFromEnvironment`, and test that the proxy is honoured |
| M3 | t02:10-14, :147; :86, :115 | The file list omits `cmd/versioncheck/main.go`. `Retry-After` is read, but GET retries use a fixed 250 ms and 1 s | Add the file. Say whether retries honour `Retry-After` |
| M4 | t03:291; t03:7, :317 | The mutant rationale is incoherent: `BuildPending` and `BuildRunning` are both non-terminal, so both keep the 3 s cadence. Two lines cite `b33d15acb`, while the plan says `406d0da95`/`b02424691` | Rewrite the mutant as "`Pending` read as terminal". Fix the commit references |
| M5 | t03:248; t08:34-35; t12:52-53 | Three problems. Legacy runs can carry an **empty** environment label (`pipeline.go:2185-2189`; the portal's `SecurityScanReview.tsx:429-431`), which gives an unmatched group titled `""` that cannot be drilled. 7P-8's "unverified" production label is answerable from source: it is `prod` (`pipeline.go:196-206`, `:237`). And `feat-evidence-uplo` is not what `sanitizeBranchEnv` produces for `feat/evidence-upload` (20 characters, so not truncated) | Name the empty group ("no environment recorded") and join it by branch. Move the `prod` point to settled. Use real names in the test data |
| M6 | t04:130, :183 | `SetBranchAutoDeploy`'s response is marked unverified, but it is readable: a `data` envelope with the GET's keys, `enabled` as a bool (`projects.go:4192-4198`) | Mark it verified |
| M7 | t04:150; t11:103 | The not-enforced message embeds `reconcileErr` (`err.Error()`, `project_security.go:836-839`) and is rendered verbatim | Internal error text reaches the UI, against 7P-12's reasoning. Show the fixed part only, or trim after the first `(` |
| M8 | t04:176; t11:81-91 | Reading a 500 sentence to spot an unknown grantee contradicts 7P-12's "never parse" rule | It fails safe, so keep it. Name it in 7P-12 as the single exception |
| M9 | t05:247 vs :172; t05:228 vs :152 | A nil `DeployConnection` answers "503 `deploy_not_connected`", but that code is 409. `TestLockedOriginRefusesAnother` expects 400, and the route lists no 400 for this case | Pick one status for each, and list them in the wire |
| M10 | t05:42 | The origin row `PanaceaApp/panacea/current` is not in Hub contract §9.1's row table (:1175) | Add it there, or keep the origin in a `statepath` config file |
| M11 | t07:96, :125; t08:36 | Releases are read up to 200 at a time. A deployment older than the 200th release renders *"no brief is recorded"* | A real pin reads as absent, which inverts P9. Render "pin not loaded", or query `pipelineRun=` for rows outside the page |
| M12 | t07:125-128 | The Hub's `release.*` feed events (Hub contract §9.3:1368-1376) are not wired to `Watcher.Nudge`. A teammate's rollback appears up to 60 s late | Have the Hub change poller nudge the watcher on `release.*` |
| M13 | t07:142-146 | The failure rule covers `KindUnavailable` and `KindUnauthorized` only. `KindNotFound` or `KindForbidden` from `Environments` (the project was deleted, or access was revoked) is unspecified | Say that it re-resolves the binding and never shows stale data as current |
| M14 | t07:68-70 | One goroutine, and a GET retried twice at 15 s per attempt, lets one hung read block the loop for about 46 s. The 3 s active cadence cannot hold | Put a per-cycle deadline on the loop |
| M15 | plan:666 | This row says that without `githost` "the connect screen asks for the Panacea project by name (part 06)". Part 06 has no such path, and `Choose` rejects anything outside the candidates (t06:95-99) | Build the fallback, or correct the row |
| M16 | t06:86-89 | `Resolve` calls `Deployer.Projects` once per local project per cycle, so N local projects means N `GET /api/v1/projects` every 15 s | Share one `Projects` read per cycle |
| M17 | t06:57; t09:158; t14:66 | A repository that is not a Hub project turns Ship off, and nothing tells the user how to link or create one (the Hub has `CreateProject`, Hub contract:705) | Name the route or screen that fixes it |
| M18 | t08:143; t08:147-148 | The rollback's "for this environment" must use `SameEnvironment`: a rollback reserves with the label `prod`, while the row is `production`. Pending rows also never expire, so a confirmed run whose deployment never appears is "pending" forever | Join through `SameEnvironment`. Put an age cap on pending rows and say "not found" |
| M19 | t08:89; t12:13, :15, :122; t13:314; t15:89 | Counts and lists in the prose disagree with the code: `promote` is not a trigger Panacea sets (plan:219-221); "six" drill routes, where there are seven; "three" `classify` callers, where there are four (verified); a "seven-state" pin cell with eight rows | Correct each |
| M20 | t09:61-74, :121-132, :251 | `ship.Config` has no Briefs dependency, yet `Readiness` calls `Briefs.ForBranch`. `PinPreview` also re-implements the Hub's pin rule on the client | Add `Briefs` to `Config`. Keep the preview labelled a preview, and test it against the Hub fake's pin selection |
| M21 | t09:174-183 | A `KindUnavailable` with status 429 or 3xx matches no abandon branch | Abandon on every `KindUnavailable` |
| M22 | t09:148-150; Hub contract:1336 | The desktop's `Idempotency-Key` guards only the control plane's in-memory cache. `hubclient` mints its own key per call, so a control-plane restart between reserve and the desktop's retry reserves twice | Derive the Hub key from the desktop's, or state that the first reservation expires |
| M23 | t10:82-83, :144 | Rebuild falls back to `source.Branch`, the sanitised label (I4) | Drop the fallback: with no Hub `branchRef`, refuse with `no_branch` |
| M24 | plan:135 vs t18:96-102 | B3 says access writes are held for 5 s. Task 18 holds only `none` and `password`, revoke and releasing an address | The refinement is well argued. Make plan:135 say what task 18 does |
| M25 | t09:151 vs t10:95; t10:31-40 | Ship locks per branch, while rollback and rebuild lock per environment, so a Ship and a rollback of one environment can race. `withReservation` also reuses Ship's abandon sentence (*"could not start the build"*) for a failed rollback | Use one per-environment lock (resolved as in C2), and a sentence per act |
| M26 | t16:108; t05:206 | After a reserve, `upstream-down` says *"the release was not started"*. On a timeout (t09:178-183) Panacea may have started it | Carry `details.status` on `upstream_unavailable` too, and use the ambiguous sentence when the status is 0 |
| M27 | t15:146-147, :163 | With phase 4's stub, every pin reads `resolving` forever, which looks the same as a slow tick | Give the stub a "not wired" state, or show a one-line note |
| M28 | plan:67-73; t20:35 | S7-N1 says the lift "swaps one line in `main.go`". But `connection.Manager` (the `Version` and `pnc_` checks), `/v1/deploy/connection`, `ConnectPanel` and `Account.Approved` are Panacea-specific and sit **above** `Deployer` | List them as part of the swap surface, so the lift is budgeted honestly |
| M29 | t19:106-110, :188 | The "previous release" rule skips rollbacks. After d1 (c1), d2 (c2), a rollback to d1, then d3 (c3), the range is c2..c3. But c1 was serving before d3, so the sessions between c1 and c2 that d3 re-ships are not credited | Take the previous release as whatever was **current** just before the row (the I3 rule) |
| M30 | t11:68-69; t18:81, :138 | Revoke by grantee removes the person from **every** role group (`keycloak.go:245-257`), and Grant adds one group, so a role change leaves the old role in place. A legacy group row (empty `grantee`) has no revoke path | Define a role change as a held Revoke followed by a Grant. Revoke a legacy row by its `group` |

---

## Assessment

This is a strong plan. The provenance mechanics (reserve → deploy → confirm or abandon, and the
join by run name), the rollback key under R5, the `pnc_` handling, the swap seam and the design
system discipline are all sound, and every claim is sourced. The four highest-risk seams,
though, are exactly where Panacea's data is not what the plan assumed:
- the environment's `branch` is the environment key (C2);
- `security-status` defaults to production (C3);
- `Deployment.branch` is a sanitised label (I4);
- the budget gate answers 402 (I2).

Add the transport gap (C1) to those. None of the five fails a test the plan writes: every fake and
recorded body was written from the same assumption. They would first appear in the owner's
batched session, where each one costs a round trip.

**The two defects a first implementer would most likely hit:**
1. **I1, the acceptance greps.** Certain, and early: task 01's own acceptance (t01:581) cannot
   print what it promises, and tasks 03, 04, 05 and 07 repeat the pattern.
2. **C1, the CORS preflight.** Certain, the first time the lens runs in the Tauri window: the
   connect panel's `PUT` and Ship's `Idempotency-Key` are refused before any handler runs, while
   every test is green.

**The two that survive every test and cost the most:** C2 (Ship refused for almost every real
branch) and C3 (the wrong environment's scan verdict shown as this release's).

**Fix order, and effort:**
1. C1 and I9. About 30 minutes: one constant pair with tests, and one deleted step.
2. C2 and I8. About 2 hours of plan edits to tasks 03, 07, 08, 09, 15 and 17, plus reframing
   HA7-1, which C2 shows is no longer blocking.
3. C3. About 1 hour: the interface, the connector, two routes and the test data.
4. I2, I3, I4, I5, I6 and I7. About 3 hours in total.
5. I1 and the Minors, while editing the parts they sit in.

Once these are made, the plan is ready for implementation without another full review. A targeted
re-check of tasks 03, 08, 09 and 15 is enough.

**Plan quality: Needs fixes**

---

## Fix batch b01 — C1 and I9 (2026-09-24)

Both findings are fixed as the review said. Two points were changed on the facts, and are
explained below the tables. Line numbers are those after the batch. `t05` means
`2026-09-23-phase-7-deploy-parts/task-05.md`, and `plan` means `2026-09-23-phase-7-deploy.md`.

**Re-verified against the tree before editing:**
- `origin.go:168-172` holds `corsMethods = "GET, POST, PATCH, DELETE, OPTIONS"` and
  `corsHeaders = "Content-Type"`, and `:231-237` serves them.
- `origin_test.go:171-172` pins both values **exactly**, in `TestMiddlewareAnswersPreflightBeforeTheMux`
  (`:151-182`).
- Hub `task-37.md:154-159` writes both constants as literals: `"Content-Type, Authorization"`,
  and methods without `PUT`.
- Phase 2 fixes the values at `phase-2-parts/task-09.md:5`, `:642-643` and `:952-953`, and at
  `task-24.md:1299`.

### C1 — the preflight refuses `PUT` and `Idempotency-Key`

| Change | Where |
|---|---|
| `origin.go` and `origin_test.go` added to task 05's file list | t05:21-22 |
| New section: why, both constant pairs (with Hub C3 landed and without it), the comment rewrite, and the declared amendments to phase 2 and Hub C3 | t05:170-217 |
| `TestPreflightAllowsPut` and `TestPreflightAllowsIdempotencyKey` (membership rows); the existing exact row at `origin_test.go:171-172` updated | t05:289-291 |
| Why the rows test membership, and the two "constant unchanged" mutants | t05:296-302 |
| Steps: the rows go red first; the amendment gets a commit of its own | t05:307-310 |
| Acceptance: `grep` of the two constants | t05:329-330 |
| Not claimed: WebView2's own behaviour is live-only (LIVE-CHECK steps 2 and 7) | t05:340-342 |
| Task 07's `origin.go` line now says it touches `Expose-Headers` only | t07:17 |
| The file map row | plan:484 |
| The task table row 05 | plan:526 |
| The §10.1 seam row declaring the amendment and the land-second rule | plan:666 |
| LIVE-CHECK steps 2 and 7 name the two preflights they exercise | t20:394, t20:399 |

### I9 — task 02's live acceptance step

| Change | Where |
|---|---|
| `cmd/versioncheck/main.go` added to the file list (this also closes M3's first half) | t02:15 |
| The live-run step replaced: write the command, do not run it, and say where 7P-10 is settled | t02:146-153 |
| Acceptance: `go run … https://dev.panacea.kpmg` replaced by `go vet` of the command | t02:162-163 |
| "Does not claim" row: no live call is made in this task | t02:172-174 |
| 7P-10's evidence line now points at LIVE-CHECK step 1, not at task 02 | plan:293-295 |
| §12's two restatements ("Task 02 … re-check", "task 02's acceptance has the one live call") | plan:716-720 |
| LIVE-CHECK step 1 "Settles": the command is task 02's; the run is the owner's | t20:393 |

### Deviations from the review's "how to fix", declared

1. **The review says "both asserting the allow lists." The new rows check membership, not the
   whole string.** Hub task 37 rewrites the same constant. A row that pinned the whole string
   would be rewritten by whichever stream lands second, and could drop the other stream's entry
   without anyone noticing. Phase 2's existing exact row stays and is updated. The membership
   rows are what keep `PUT` and `Idempotency-Key` in place in either landing order.
2. **The review did not name the existing exact row at `origin_test.go:171-172`.** Changing the
   constants turns it red, so task 05 updates it on purpose instead of meeting it as a surprise.

### Left open

- **Hub `task-37.md:154-159` still shows literal constants** that would drop `PUT` and
  `Idempotency-Key` if task 37 lands after task 05. That plan is in the Hub stream's worktree,
  which this stream may not edit. Phase 7 declares the rule at plan:666 and t05:212-217, and the
  membership rows catch a violation. The Hub stream should change its code block to "add
  `Authorization` to the list you find". Relayed through the orchestrator.
- **M3's second half (`Retry-After` against fixed GET backoff) is not in this batch.** It stays
  with the Minors.

---

## Fix batch b02 — C2 and I8 (2026-09-24)

Both findings are fixed with the review's mechanism: Panacea's own mapping plus `SameEnvironment`,
and no join on raw names anywhere. Four points were changed or added on the facts, and are
explained below the tables. Line numbers are those after the batch. `tNN` means
`2026-09-23-phase-7-deploy-parts/task-NN.md`, and `plan` means `2026-09-23-phase-7-deploy.md`.

**Re-verified against `~/Projects/Panacea/services/panacea-api` (read-only) before editing:**
- `routes.go:1430`: `GET …/branches/{branch}/access` is `authedProject` (member-readable).
- `handlers/branch_security.go:29-37`: `branchEnv` → `mapBranchToEnvWithProd`.
  `project_security.go:636` reads `r.PathValue("branch")`, `:646` calls `branchEnv`, and
  `:668-677` writes `data.env`. No existence check. `:641-644` is a 503 without a k8s client.
- `handlers/projects.go:2109-2118`: *"For non-base envs Env==Branch already"*, and the row writes
  `"branch": e.Branch`. `:3783-3826`: `mapBranchToEnv` and `sanitizeBranchEnv`, which cut at 20
  characters. `:3838-3849`: `mapBranchToEnvWithProd`.
- `clients/pipeline.go:237`: `env := environmentForBranch(branch, prodBranch)`, a function at
  `:2195-2207`. `:608` writes it as `panacea.io/environment`. `:609` writes
  `panacea.io/branch: sanitizeLabelValue(branch)`, a function at `:2351-2364`.

### C2 — `environments[].branch` is the environment key, not the git branch

| Change | Where |
|---|---|
| `Reader.EnvironmentFor(ctx, p, branch)` is the only branch → environment mapping, joined with `SameEnvironment` | t01:61-67 |
| `Environment.Branch` is documented as Panacea's label, never a join key | t01:139-143 |
| `Fake.SetBranchEnvironment`: seeded, derives nothing, and an unseeded branch fails loudly | t01:514-518 |
| `Seeder.BranchEnvironment` | t01:540-542 |
| Conformance row `EnvironmentForIsTheTargets`: `feat/x`, a 31-character branch, and `main`, with no row | t01:569; "seven rows" t01:591 |
| `environmentDTO.Branch` is commented as a label | t03:61 |
| `branchEnvDTO {Env}` | t03:71-75 |
| The route row: `branches/{branch}/access`, `PathEscape`, `data.env`, the empty-answer rule, and the 503 | t03:207 |
| `Seeder` gains `BranchEnvironment` | t03:242-244 |
| Recorded bodies: `environments.json` row `feat-evidence-upload` with `branch == name`, and three `branch-access-*.json` | t03:256-257 |
| `panaceatest.Server` answers from a seeded table and holds no naming rule | t03:284-288 |
| `TestEnvironmentForReadsPanaceasName` (`feat%2Fx`, long, `main`, empty) and `TestEnvironmentForJoinsProduction` | t03:309-310 |
| Problem code `deploy_environment_invalid` replaces `deploy_environment_unknown` | t05:230, t13:254 |
| The disconnected-method count is 24 | t05:282 |
| `Watcher.WantBranch` | t07:81-85 |
| `Snapshot.BranchEnvs` | t07:97-101 |
| Cycle step 2: `EnvironmentFor` once per new wanted branch, re-read every fourth cycle | t07:127-130 |
| `TestEnvironmentsBeforeLatest` amended; `TestWantedBranchResolvedOnce` | t07:189-190 |
| The view route takes `?branch=` and calls `WantBranch`, with no request | t08:21-23 |
| `DeployView.selected {branch, target, environment}` | t08:36-40 |
| The example row is `feat-evidence-upload`, with its label; `branchesSeen` holds run labels | t08:49-51, t08:54 |
| Rule 1a, *Selected branch*: never compare a git branch with any Panacea label | t08:143-149 |
| Rule 9: labels are compared only with each other | t08:170-171 |
| Tests: slash, long, production, never deployed | t08:184-187 |
| `TestViewHandlerMakesNoCall` covers `?branch=` | t08:201 |
| The read-only catalogue line | t08:203-205 |
| Task 09's header: no Hub dependency | t09:3-6 |
| `Tracker.WantBranch` and `EnvironmentNamer` | t09:60-67 |
| `Config.Environments` replaces `EnvironmentOptional` | t09:77-79 |
| Readiness `Environment` comes from `BranchEnvs`, with reason `environment_invalid` | t09:125-131 |
| Readiness example value | t09:152 |
| Step 4: `EnvironmentFor` before reserve; the error mapping; the §7.9 check; reserve with the answer verbatim | t09:178-196 |
| Wire: `409 deploy_environment_invalid` | t09:235 |
| Six tests replace `TestEnvironmentUnknownUntilHA71` | t09:259-264 |
| TS types: the label comments, `environmentRow`, `selected` | t13:68-71, t13:113, t13:154 |
| `view(projectId, branch)` sends `?branch=` verbatim, and `loadDeployView` sends the selection | t13:208-209, t13:303-306 |
| Test `view sends the branch verbatim` | t13:326 |
| The card finds its row through `view.selected.environment`, in three states | t15:67-80 |
| The collision banner fires on `branchesSeen` only, not `env.branch !== branchName` | t15:127-135 |
| Five tests: slash, long, production, never deployed, and a source grep for name compares | t15:153-157 |
| Hub-ask row HA7-1: withdrawn | t20:49 |
| The live test reads `EnvironmentFor("live-check/p7")` | t20:176-185 |
| LIVE-CHECK step 6 settles the `%2F` read | t20:406 |
| 7P-8: the production label is `prod` on the deploy path (from source) | plan:269-273 |
| 7P-8: one join, in the control plane; a git branch is never compared with a name; Panacea's mapping; a first Ship is knowable | plan:278-291 |
| §4 row 15: `data.env` and its users 03, 04, 07, 09, 11 | plan:376 |
| §8 row 09: no Hub ask | plan:543 |
| §9 HA7-1 reframed as **withdrawn**, with the false premise (plan:563-564 before) and the old *meanwhile* (plan:579-583 before) named | plan:575-590 |
| §12: the production label settled from source; the `%2F`-through-ingress question added | plan:725-733 |

### I8 — the drawer re-joins environments by exact name

| Change | Where |
|---|---|
| `ReleaseRow.environmentRow`: the matched row's name, or `null` when unmatched | t08:88 |
| `ReleaseRow.environment` stays the raw label (`prod`) | t08:87 |
| Rule 1: every `ReleaseRow`, and Hub rows too, carries `environmentRow` | t08:140-142 |
| `TestReleaseRowCarriesItsRow` (a `prod` deployment, a `prod` rollback, an unmatched row) | t08:188 |
| TS `ReleaseRow.environmentRow` | t13:113 |
| The drawer looks up `row.environmentRow`; an unmatched row gets no drill-ins | t17:121-128 |
| Tests `prod row keeps its drill-ins` and `unmatched row has no drill-ins` | t17:218-219 |

### Deviations and additions, declared

1. **The review names the route but not how the desktop gets its answer.** The view handler makes
   no network call (t08:5-6). So the desktop sends `?branch=`. The watcher's cycle resolves the
   branch (`WantBranch`, at most 16 per project), and the view returns `selected`. Ship reads
   `EnvironmentFor` directly at step 4, because an act should use Panacea's answer now, not a
   cached one.
2. **`deploy_environment_unknown` is replaced, not deleted.** `sanitizeBranchEnv` keeps characters
   such as `.` (`projects.go:3814-3826`), so `release/1.2` becomes `release-1.2`, which §7.9's
   `[a-z0-9-]` refuses. The new code, `deploy_environment_invalid`, checks the Hub's published
   rule, not Panacea's naming rule.
3. **`Deployment.branch` is also a sanitised label, not the git branch.** It is
   `sanitizeLabelValue(branch)` (`clients/pipeline.go:609`). This settles the question t08 had
   left *unverified*. The collision banner therefore no longer claims which branch is serving now.
   It lists the labels, because comparing a label with the git name would need a copy of
   Panacea's rule.
4. **Three facts corrected.**
   - The plan's example `feat-evidence-uplo` was wrong. `feat-evidence-upload` is exactly 20
     characters, and the cut applies only past 20.
   - Review I8 (line 307) and the Important at line 261 cite `pipeline.go:196-206`. The function
     is at `clients/pipeline.go:2195-2207`, and `:237` is the call site. The parts cite the
     correct lines. The review's own text is left as written.
   - Ship now reserves with `prod`, Panacea's answer for a production branch. Before, it reserved
     with the row name `production`. Rollback also reserves with the label `prod`, so the Hub
     holds one spelling.

### Left open

- **Unverified: whether the ingress passes `%2F` to Panacea's handler.** Go's `ServeMux`
  unescapes a wildcard segment (per the `net/http` docs; not run here). Traefik was not checked.
  It is settled live by LIVE-CHECK step 6 (t20:406) and `TestLiveEnvironmentsAndBranches`
  (t20:176-185). If it fails, Ship refuses with Panacea's error. It never guesses.
- **The HA7-1 withdrawal is relayed to the Hub stream through the orchestrator.** If the Hub has
  already built a nullable `environment`, it does no harm, because phase 7 never sends `null`.
- **Not in this batch:**
  - M18: rule 6's rollback *"for this environment"*, and the age cap on pending rows. Rule 1
    now gives rollback rows `environmentRow` through `SameEnvironment`.
  - M25: one lock per environment. It can key on the name `EnvironmentFor` resolves.
- **b01's line numbers moved with this batch.**
  - plan:293 → 306, :484 → 497, :526 → 539, :666 → 670, :716-720 → 720-724.
  - t20:393 → 401, :394 → 402, :399 → 407.
  - The t02, t05 and t07 citations are unchanged.

### Fix batch b02, round 2 — the re-review's R1 to R9 (2026-09-24)

Source: `.superpowers/sdd/p7/plan-rereview-b02-1.md`. Commits `06bb704`, `d3342fc`, `061cb91` and
the two after it. Line numbers are those after round 2.

**Re-verified against `~/Projects/Panacea/services/panacea-api` (read-only) before editing:**
- `handlers/branch_security.go:29-37`: `branchEnv` reads only the `production-branch` label (`:32`).
- `clients/pipeline.go:233-237`: the deploy calls `ResolveProjectProductionBranch` (`:236`).
  `:609` writes `sanitizeLabelValue(branch)`, a function at `:2351-2364`. It replaces `/` and `_`
  with `-` and keeps capitals.
- `clients/kubernetes.go:5189`: the environments list calls `ResolveProductionBranch`. That
  function is at `:10179-10204`: label, then annotation, then `main`, and it skips `prod` and
  `production`.
- `clients/kubernetes.go:4210-4217`: `tenantNamespace` sends `prod`, `production` and `""` to the
  base namespace. The round-1 re-review had not re-read this.
- `handlers/projects.go:3838-3849`: with an empty `prodBranch`, `develop` maps to `develop`.

| Finding | Change | Where |
|---|---|---|
| R1 | `EnvironmentFor` is the access route's mapping, which reads the label only | t01:67, t03:207 |
| R1 | Fixture `branch-access-annotation-only.json`, and `TestEnvironmentForIsTheAccessRoutesMapping` | t03:258, t03:317 |
| R1 | Rule 5a: `environmentMismatch` from facts after the deploy, never joined by the Hub's name | t08:106-109, t08:170-178 |
| R1 | Test `TestReleaseRecordedAgainstAnotherEnvironment` (`develop` against `prod`) | t08:230 |
| R1 | Ship does not pre-check the primary row's `branch` (step 3 of the fix) | t09:201-203 |
| R1 | TS `environmentMismatch`; the card draws the sentence; test `mismatch is drawn` | t13:114-115, t15:96-99, t15:186 |
| R1 | 7P-8, §9 HA7-1, Q-P7-6 and §12 name the Panacea defect | plan:296-303, plan:595-599, plan:723, plan:756-765 |
| R1 | Live row: LIVE-CHECK step 5a and `PANACEA_APP_LIVE_PROD_BRANCH` | t20:69-70, t20:188-198, t20:419 |
| R2 | `BranchEnvErrs`, retried every fourth cycle, never staling the snapshot | t07:106-109, t07:136-143 |
| R2 | Test `TestRefusedBranchNotReaskedEveryCycle` | t07:206 |
| R2 | `selected.error` (Go and TS) and rule 1a | t08:40, t08:151-155, t13:160-161 |
| R2 | The route's 404 is rewritten to the `/` sentence; `TestEnvironmentForRouteNotFound` | t03:316 |
| R2 | Ship: `409 deploy_environment_unreadable`, never `deploy_not_found`; `TestShipEnvironmentUnreadable` | t05:231, t09:185-188, t09:246, t09:275, t13:263 |
| R2 | The card's refusal state and test `refused names the refusal` | t15:86-90, t15:184 |
| R2 | §12: the `%2F` path ends in a kept refusal and a card sentence | plan:748-755 |
| R2 | LIVE-CHECK step 6 names the refusal sentence as the `%2F` failure | t20:420 |
| R3 | `panaceatest.Server` answers the route from both seeds; `env` is omitted for a posture-only branch | t03:285-294, t04:187-192 |
| R4 | `Assemble(s, same, now)`; `Watcher.SameEnvironment`; the view tests' `prod`/`production` stub | t07:87-89, t08:144-147, t08:204-209 |
| R4 | Test `TestWatcherPassesSameEnvironment` | t07:207 |
| R5 | The props are `selected` / `selectedEnvironment`, and the git branch is display text only | t15:59-72 |
| R5 | The `no branch comparisons` grep covers both operand orders, `includes`/`startsWith`/`endsWith`, and the old prop names | t15:187 |
| R6 | `onDeployCursor` goes through `loadDeployView`; `cursor change loads once` asserts `?branch=` | t13:307-311, t13:334 |
| R6 | The card's `selected == null` state and test `no selection, no card` | t15:83-85, t15:185 |
| R7 | The collision fixture uses run labels | t08:228 |
| R7 | Rule 9 names the blind spot | t08:191 |
| R7 | The banner's blind spot, cited | t15:157-162 |
| R8 | Rebuild's branch is the Hub `branchRef` only, else `409 no_branch` | t10:82-87 |
| R8 | Test `TestRebuildRefusesLabelOnlyBranch`; `TestRebuildPrefersHubBranchRef`'s note corrected | t10:148-149 |
| R8 | 7P-3's Rebuild body | plan:204-208 |
| R8 | The desktop offers Rebuild only with a Hub release; test `rebuild needs a Hub release` | t16:96, t16:105-108, t16:141 |
| R9 | Roll back finds *its environment* through `environmentRow`, and fails closed | t16:98-103 |
| R9 | Test `rollback off while production builds` (`prod` against `production`, and unmatched) | t16:140 |
| (R1, t19) | *"The previous one"* compares `environmentRow`, not the raw environment | t19:108 |

**Declared, on the facts:**
- **R2 step 6, the 404 rewrite:** kept as `KindNotFound` with the rewritten sentence, not as
  `KindUnavailable`. `KindUnavailable` would stale the whole cycle for every project (t07's failure
  rule) over one branch name. Ship maps it to its own code, `deploy_environment_unreadable`.
- **R5:** `EnvironmentCard` takes the whole `view.selected`, not `selectedEnvironment`, because
  its five states need `target` and `error` too. The branch reaches it as `selected.branch`,
  display text.
- **R7 blind spot, stated precisely:** twins that differ only in `/`, `_` or `-` share one label.
  Twins that differ in case do not, because the label keeps capitals and the environment is
  lower-cased. Twins that differ past the 20th character do not either, because the label is cut
  at 63. The banner shows both of those.
- **R8, carried into t16 and 7P-3:** the desktop has no `branchRef` (the TS `ReleasePin` does not
  carry one). So it offers Rebuild on `release != null`, and it shows the control plane's
  `no_branch` refusal verbatim for a Hub release without a `branchRef`.
- **R9:** two disabled sentences were added: *"`<env>` has a build running. Roll back once it
  finishes."* and the unmatched-row line from t17.

**Relay through the orchestrator (not sent from here):** tell the Panacea owner that `branchEnv`
should call `ResolveProjectProductionBranch`, as the deploy and posture paths do (Q-P7-6).

**Unverified:** whether Traefik passes `%2F`. How many live projects are annotation-only or carry
a `production` mislabel (LIVE-CHECK step 5a counts the live project only). Nothing was compiled or
run. This is plan text.


## Fix batch b03 — C3 and I4 (2026-09-24)

Both findings were re-checked against Panacea at `~/Projects/Panacea` (read only) before any edit.
C3 holds as written: `resolveEnv` defaults `""` to `"production"` (`handlers/security_status.go:192-201`),
`collectScansFromK8s` lists `tenantNamespaceForEnv(project, env)` (`:495`), which sends `""`, `prod`
and `production` to the base namespace (`handlers/tier_store.go:106-113`), and keeps only ConfigMaps
whose `panacea.io/env` matches (`:526-534`). The portal sends `env` only when non-empty
(`backstage/packages/app/src/plugins/SecurityScanReview.tsx:437-440`). I4 holds as written
(`clients/pipeline.go:609`, `:2351-2364`; `clients/kubernetes.go:5750`), with one citation
corrected: `environmentForBranch` is at `pipeline.go:2195-2207`, not `:196-206`.

### C3 — scans are read in the deployment's environment

| Change | Where |
|---|---|
| `Scans(ctx, p, env, deploymentID)`, with the why | t01:78-84 |
| `Deployment.Environment` notes that Panacea's production word is `prod` and that Scans takes it | t01:177 |
| `Fake.SetScans(p, env, deploymentID, r)`, keyed by the exact env; unseeded → `KindNotFound` | t01:532-535 |
| `Seeder.Scans(p, env, deploymentID, r)` | t01:561-563 |
| Conformance row `ScansReadTheDeploymentsEnvironment` (kills the drop-env mutant over both implementations) | t01:588 |
| Route row: `?env=&deployId=`, `env` set with `url.Values` when non-empty | t03:211 |
| `panaceatest.Server.Scans`: a missing `env` (or `prod`/`production`) answers production's report | t03:296-305 |
| `TestScansSendsTheEnvironment` (query shape; no `env=` when empty) | t03:330 |
| Watcher step 5 calls `Scans(ctx, p, d.Environment, d.ID)` | t07:151-156 |
| `TestScansReadWithTheDeploymentsEnvironment` | t07:212 |
| Drill route `GET {P}/scans?environment=&deploymentId=`: `environment` required as a key, echoed in the body, and the empty-value note | t12:60-79 |
| `TestScansPassTheEnvironment` | t12:128 |
| TS `ScanReportView.environment`; `DeployApi.scans(projectId, environment, deploymentId)` | t13:189, t13:225-226 |
| Test `scans send the environment` | t13:343 |
| `ScanList` calls `api.scans(projectId, row.environment ?? "", row.deploymentId)` | t17:160-163 |
| Test `scans read in the row's environment` | t17:220 |
| `LIVE-CHECK` row 8 builds its query with `url.Values` and the latest deployment's `environment` | t20:284-301 |
| §4 row 8 route shape and users | plan:385 |
| Recorded as settled | plan:802-809 |

### I4 — `Deployment.branch` is a label value

| Change | Where |
|---|---|
| `Deployment.Branch` documented as a label value, never sent as a branch | t01:178-183 |
| Settled in plan §12 (new subsection *Settled from source, no longer open*), with the `release/x` consequence closed | plan:787-801 |
| Collision banner: *"Panacea records them only by label — … — not by their git names"*; never a link, never offered to Ship or Rebuild | t15:150-156 |
| Test `collision banner words` asserts the label wording and no `<a>` or button | t15:182 |
| Rebuild's comment cites I4, M23 and plan §12 | t10:87-88 |

**Already done by b02 (no edit needed, checked):** `branchesSeen` is worded as run labels
(t08:52, t08:187-196); t08's "unverified" point is settled at t08:253-261; Rebuild's
`source.Branch` fallback was dropped in b02 round 2 R8 (t10:82-87, test t10:150), which closes
M23. The review's line numbers t08:151-153 and :197-202 predate b02 and now point at rules 1a, 10 and 11.

### Deviations from the review's "how to fix", declared

- **The drill route requires `environment` as a key but allows an empty value.** A required key
  stops a desktop caller from forgetting it (the C3 mutant one layer up). An empty value is legal
  because a PipelineRun without the `panacea.io/environment` label reads back `""`
  (`kubernetes.go:5681`, `:5751`); Panacea then answers production's report, as its own portal
  does. Whether any live run lacks the label is unverified.
- **`panaceatest` copies one Panacea rule** — `""`, `prod` and `production` are production — because
  it is the rule the C3 mutant hides behind. The Fake copies nothing and keys by the exact string.
- **Added beyond the review:** the watcher test (t07), the desktop client and scan-list tests
  (t13, t17) and `LIVE-CHECK` row 8 (t20), because each restated the old two-argument call.

### Mutants

This batch changes plan text; no phase 7 code exists in the worktree
(`control-plane/internal/deploy/` is absent), so no mutant was run. The plan now names the
drop-env mutant at four layers: the connector (t03 `TestScansSendsTheEnvironment` and the
conformance row over `panaceatest`), the watcher (t07), the drill route (t12) and the desktop
(t13, t17). The two I expect most likely to survive, once code exists: sending `env=` with an
empty value (killed by t03's *no `env` key* assertion), and the scan list sending `environmentRow`
instead of `environment`. For a production row that is `production` for `prod`, which Panacea
answers identically, and for a matched branch row the two are equal; it goes wrong only for an
unmatched row (`environmentRow: null` → `""` → production). So it is killed only by t17's explicit
`"prod"` assertion, and task 17's mutation step must run it.

### Left open

- Whether any live PipelineRun lacks `panacea.io/environment` (task 20 can count it).
- `ReleaseRow.branch` mixes a run label (run rows) and a Hub `branchRef` (Hub rows) in one field
  (t08:90). It is display-only and never compared (t15's `no branch comparisons` test), so it is
  not a defect today; splitting it is a Minor, deferred as out of scope for C3 and I4.
- Nothing was compiled or run. This is plan text.

### Fix batch b03, round 2 — the re-review's I-1, M-1 and M-2 (2026-09-24)

Input: `.superpowers/sdd/p7/plan-rereview-b03-1.md`. Panacea re-read at `~/Projects/Panacea`
`70b257893`, read-only. Every new citation below was re-read at source in this round:
`webhook-receiver/pipeline/trigger.go:1268`, `:1270`, `:1546-1551`, `:1553-1574`;
`webhook-receiver/models/event.go:116-148`; `panacea-api/clients/pipeline.go:842`, `:2195-2253`,
`:2351-2364`; `clients/kubernetes.go:5595-5602`; `handlers/security_status.go:195-207`.

**I-1 (Important) — Panacea has two label writers; the banner counted labels as branches.**
Confirmed at source: the webhook's `sanitizeLabel` keeps `_`, maps `\`, `:` and space to `-`,
trims every non-alphanumeric end and answers `""` when empty; the API's `sanitizeLabelValue` folds
`_` into `-` and answers `"unknown"`. Fixed as the re-review says; the labels are **not**
normalised.

| Change | Where |
|---|---|
| §12 records both writers, their rules, the `feat_x` split, and "labels as written, never normalised" | plan:792-814 |
| §5 and §4 Rebuild restatements name the second writer | plan:207-209, plan:289-291 |
| S7-N3: a hash on truncation does not stop the push-and-Ship split firing | plan:705, t20:37 |
| `Deployment.Branch` comment names both writers and `feat_x` → `feat_x` / `feat-x` | t01:178-188 |
| `branchesSeen` / `collision` JSON comments: labels verbatim, collision = more than one **label** | t08:52-53, t08:90 |
| Rule 1a: whether `_` survives depends on the writer | t08:156-160 |
| Rule 9: two writers; false positive (one branch pushed then Shipped); blind spot restated per writer | t08:188-205 |
| New test `TestCollisionKeepsPushLabel`: `feat_x` + `feat-x` → `collision: true`, `_` intact | t08:238 |
| "Does NOT claim": two writers; new bullet "That `collision` counts branches" | t08:264-274 |
| Rebuild comment: `source.Branch` may be either writer's label | t10:85-90 |
| Rebuild row text names the webhook citation | t16:106-108 |
| Banner reworded to the re-review's text: *"…under more than one branch label: … A Ship and a push write the label differently … If these are different branches, …"*; the desktop never folds `_` | t15:148-164 |
| Blind spot restated per writer; "both writers keep capitals" | t15:165-171 |
| `collision banner words` anchors on the new phrase | t15:188 |
| New test `push and Ship labels are not two branches`: `["feat-x","feat_x"]` → banner drawn, `feat_x` intact, the DOM lacks *"more than one branch."*, *"deployed from more than one branch"* and *"two branches"* | t15:189 |
| Live check logs `withUnderscoreInBranch` (the live sign that push runs reach the list); LIVE-CHECK step 3 copies it | t20:172-187, t20:432 |

**M-1 (Minor, fixed; C3's false statement in a narrow case) — an empty `environment` read
production's verdict.** Settled from source that `""` is legacy-only (`pipeline.go:2195-2253`,
`:842`; `event.go:116-148`).

| Change | Where |
|---|---|
| `ScanList` takes `environment: string \| null`; `""` or null → no call, the sentence *"Panacea recorded no environment for this release, so its scans are not read."*, no **Try again** | t17:79, t17:160-169 |
| New test `no environment, no scan read` | t17:227 |
| The watcher does not call `Scans` for a deployment whose `Environment` is `""` | t07:156-158 |
| New test `TestScansSkipsEmptyEnvironment` | t07:225 |
| The route's note: stays a pass-through, the desktop never sends `""`, and the "unverified" is settled from source | t12:76-86 |

**M-2 (Minor, fixed) — the Fake's unseeded answer and the watcher's handling of it.** Took the
re-review's first option (state the watcher rule) rather than changing the Fake to an empty
`SKIPPED` report: the rule is needed live anyway (a 400 or 403 on `security-status`), and
`KindNotFound` stays a loud test-bug signal.

| Change | Where |
|---|---|
| Step 5: a `Scans` failure other than `KindUnavailable` / `KindUnauthorized` leaves `Snapshot.Scans[d.ID]` unset (`not_read`), sets no `Stale` or `LastError`, does not end the cycle, and is retried on the next `ID`/`Build` change or `HistoryInterval` | t07:159-167 |
| New test `TestScansFailureIsNotStale` | t07:224 |
| `SetScans` comment: Panacea never errors here; `KindNotFound` is a test-bug signal; the conformance suite does not exercise an unseeded pair, and panaceatest's 500 + `t.Errorf` is also one | t01:538-545 |

**Declared.** The re-review's M-2 text named no `Kind` for a body the connector cannot read; t01
has no `KindProtocol` (t01:473-481), so t07 names `KindRejected` (Panacea's 400) and
`KindForbidden` instead. t03 already maps an unreadable answer to `KindUnavailable` (t03:294),
which follows the cycle failure rule.

**Mutants for this round (plan text; none run, as there is no phase 7 code).** Named likely
survivors before writing the tests: (1) a view that folds `_` into `-` before counting — killed by
`TestCollisionKeepsPushLabel` (t08:238); (2) a `ScanList` that sends `row.environment ?? ""` —
killed by `no environment, no scan read` (t17:227). Also: a watcher that stales on a `Scans`
`KindNotFound` — killed by `TestScansFailureIsNotStale` (t07:224); a banner that keeps *"more than
one branch."* — killed by t15:189.

**Left open.** Whether the live dev estate runs the webhook-receiver for this project's pushes
(the live check's `withUnderscoreInBranch` count answers it, t20:185-187).
