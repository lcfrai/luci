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

## Fix batch b04 — I3 and I7 (2026-09-24)

I3 was re-checked against Panacea at `~/Projects/Panacea` (read only) before any edit, and holds.
`createdAt` is the PipelineRun's creation timestamp (`clients/kubernetes.go:5680`, `:5754`).
`duration` is `completionTime − creationTimestamp` rounded to a second, formatted by
`time.Duration.String()`, and `""` while there is no `completionTime` (`:5683-5689`, `:5752`;
`models/models.go:682`). The durable store keeps it (`clients/deployment_store.go:83`, `:134`). A
backfilled row has neither a duration nor an outcome (`clients/deployment_backfill.go:167-175`,
`Status "historical"`). I7 holds as written. Phase 3's desktop surface was read in the phase-3
worktree's plan (read only): `RemoteState {upstream, ahead, behind}` and `Convergence.tip`
(phase 3 t18:50-53 and :65-68), `pushBranch` (t18:208), `addGoodTickListener` (t18:218-223),
`useConvergenceLoader` with `reload()` (t18:255-265) and `selectConvergence` (t18:269) — lines
re-checked in round 2 (re-review b04-1, M-5: phase 3's t18 gained `bringIn` in 79527d6). That
loader is mounted only by Merge and Files, so nothing refreshed convergence inside the Deploy lens.

### I3 — current is the newest succeeded build by completion, or a later rollback

| Change | Where |
|---|---|
| `Deployment.CompletedAt` (`CreatedAt` + the target's duration; zero = not finished or not recorded); `CreatedAt` says it is creation, not go-live | t01:197-202 |
| Conformance row `CompletedAtSurvives` | t01:599 |
| `deploymentDTO.Duration` with its source lines; the mapper rule (`""` or unparseable → zero, never a guess) | t03:89-96 |
| `TestCompletedAtFromDuration` | t03:330 |
| `current.since` is `completedAt` or `confirmedAt`; `isCurrent` marks exactly one row, never the run a rollback returned to | t08:76, t08:102 |
| Rule 6 rewritten: `succeeded` + non-zero `CompletedAt`, latest by completion; failed/running/pending/historical/unknown never current; the later of build and rollback wins; a tie goes to the rollback; neither → `source "none"` | t08:180-200 |
| Rule 10 lists `completedAt` | t08:223 |
| `TestRollbackIsCurrent` and `TestRollbackBeforeNewerDeploy` corrected (the latter now asserts a build that **went live** after the rollback) | t08:250-251 |
| New: `TestFailedNewestIsNotCurrent` (failed and running), `TestBuildLiveAfterRollbackIsCurrent` (Ship 10:00, rollback 10:05, live 10:08 → the build), `TestCurrentTieGoesToRollback`, `TestHistoricalOnlyIsNone` | t08:252-255 |
| "Does NOT claim": completion can lag go-live (`clients/pipeline.go:1302-1306`, tasks *"beside/after the deploy"*), and a run failed by a post-rollout task is unverified | t08:296-303 |
| Plan 7P-4 restated with completion and the succeeded-only rule | plan:232-238 |
| t16:95 (now t16:123): **Roll back to this** also needs "not its environment's `current.deploymentId`", so a failed newest build no longer hides it on the release that serves, and a rollback's target is not offered a rollback to itself | t16:123 |
| Test `roll back follows what is serving` | t16:170 |
| Card line when the current deployment is not the newest build (added: rule 6 promises the card can say both) | t15:120 |
| Test `failed newest names what serves` | t15:180 |

### I7 — Ship readiness reloads when the branch's git facts move

| Change | Where |
|---|---|
| Section *When readiness reloads — four edges, no timer*: edge 4 is the selected branch's convergence `tip` / `remote.upstream` / `remote.ahead` / `remote.behind` changing on the existing good tick (`addGoodTickListener`); Push resolving reloads readiness at once and calls the convergence loader's `reload()`; a rejected push reloads nothing; a tick with no move sends nothing | t16:91-114 |
| `DeployLens` mounts phase 3's `useConvergenceLoader("merge")`, unchanged, so the value is fresh while the lens is open | t16:30 |
| Files line and step name the four edges; the loader keeps the last `{tip, upstream, ahead, behind}` it saw per branch, in memory | t16:29, t16:180-185 |
| Test `readiness on the four edges` (replaces *three edges*; ten same-value ticks → zero calls) | t16:173 |
| Test `push, then Ship turns on with no reselect` | t16:174 |
| Test `a push elsewhere turns Ship on on the next tick` (a terminal push) | t16:175 |
| "Does NOT claim": the loader's cost in Deploy is unmeasured; edge 4 sees a terminal push on the next good tick | t16:205-210 |
| Dependency on phase 3 task 18 declared | t16:3-5, plan:572 |
| Task 13's "exactly three edges" is the view's; the readiness load's fourth edge is named there | t13:8-11 |

### Deviations and additions, declared

- **The rollback's target run row is never `isCurrent`.** The review did not say which row is
  current after a rollback. The rollback row carries the flag. So t16 needs a second check,
  `current.deploymentId`, or Roll back would offer a rollback to the release already serving.
- **Ties go to the rollback.** Panacea rounds `duration` to a second, and the Hub confirms after
  Panacea's flip. The review had no tie rule. Without one, the answer could flip between cycles
  depending on list order.
- **`historical` rows are never current.** They carry no duration and no outcome. An environment
  with only backfilled rows reads `source "none"`, not a guess.
- **Added beyond the review:** the t01/t03 plumbing (the view could not see `duration` before),
  and the t15 card line, because the review's failure case — a failed newest build — otherwise
  showed only the failure.
- **I7 reuses phase 3's `useConvergenceLoader("merge")`** instead of adding a lighter `what` to
  phase 3's contract. That keeps the change inside phase 7. Its per-tick cost in Deploy is
  unmeasured (t16 does-not-claim). A lighter `what` would be a one-argument swap.

### Mutants

This batch changes plan text; no phase 7 code exists, so no mutant was run. Two named as most
likely to survive once code exists:
1. **Ordering the live candidate by `CreatedAt` while filtering on `succeeded`.** It passes every
   test where creation and completion order agree. *Corrected in round 2 (re-review b04-1, I-2):*
   `TestBuildLiveAfterRollbackIsCurrent` kills only the form that also compares `CreatedAt` with
   the rollback. The natural form — take the first `succeeded` row in Panacea's served order
   (newest by creation), then compare its `CompletedAt` with `confirmedAt` — passed every t08
   test. `TestCurrentOrdersByCompletion` (t08:254) kills it: `d1` created first and completed
   last is current.
2. **A convergence edge that compares the whole `Load` object, or `readAt`.** It fires on every
   tick and passes the Push test. It is killed by the ten-same-value-ticks clause of `readiness
   on the four edges` and the `readAt`-only clause of `a push elsewhere…`.

Also named: treating a zero `CompletedAt` as `CreatedAt` (killed by `TestCompletedAtFromDuration`
and the running half of `TestFailedNewestIsNotCurrent`), and Roll back gated on `isCurrent` alone
(killed by the rollback half of `roll back follows what is serving`).

### Left open

- Whether any Panacea task after the rollout can fail a run whose pods serve (t08 does-not-claim).
- ~~t17's **Follow readiness** is offered for an `isCurrent` run row. After a rollback, the current
  row is the `rollback` row, so no run row offers it. This is out of scope for I3, deferred as a
  Minor.~~ *Withdrawn in round 2:* the re-review (b04-1, I-1) ruled it a defect of this batch, and
  task 17 now keys both drill-ins on "serves now" (t17:118-128).
- M29 (t19's "previous release") names "the I3 rule". It can now cite t08 rule 6. Left to its own
  batch.
- Nothing was compiled or run. This is plan text.

### Fix batch b04, round 2 — the re-review's open issues (2026-09-24)

Source: `.superpowers/sdd/p7/plan-rereview-b04-1.md` (I-1, I-2, M-1 to M-5). Panacea was re-read
at `~/Projects/Panacea` (read only) for I-2 and M-4 before editing: the cross-path comment is at
`services/panacea-api/clients/pipeline.go:102-112`; `deployment_store.go:56` and `:88` keep the
earliest `CreatedAt`; `:338-341` serve newest by `CreatedAt`; `deployment_backfill.go:172` sets
`CreatedAt: r.MintedAt`. Phase 3's t18 was re-read in the phase-3 worktree (read only) for M-5.

| Finding | Change | Where |
|---|---|---|
| I-1 | *Why it is down* and *Log* are shown when the row **serves now**: `isCurrent`, or its `deploymentId` is its environment's `current.deploymentId` (the Roll back gate's key). The rollback row gets *Why it is down* and **Follow readiness**; the target's run row gets both plus **Follow build log** | t17:118-128 |
| I-1 | Test `rollback target keeps its drill-ins` (target run row, rollback row, an older row) | t17:239 |
| I-1 | The round-1 "Left open" deferral withdrawn | review:913-916 |
| I-2 | Test `TestCurrentOrdersByCompletion`: `d1` created 10:00, completed 10:20; `d2` created 10:05, completed 10:10; served order `d2` first → `d1` is current | t08:254 |
| I-2 | Mutants paragraph corrected: `TestBuildLiveAfterRollbackIsCurrent` kills only the form that also compares `CreatedAt` with the rollback; the new test kills the natural form | review:895-901 |
| M-1 | Phase 3 tasks 18 and 12 are hard dependencies of task 16 (Push and edge 4 both come from task 18); the fallback sentence is deleted; the Push condition no longer names task 12 | t16:3-8, t16:89, plan:572, plan:582-585 |
| M-2 | Baseline rule: an edge-4 load records the value that moved; any other load clears the baseline and notes when it was sent; the first ready value with `Load.at` at or after that is recorded without a reload | t16:106-107, t16:117-128 |
| M-2 | Edges test extended: one good tick after a lens open, and after Push (old value, then the `reload()` value), each make zero further calls; two successive moves call once each; the terminal-push test names its baseline | t16:187, t16:189 |
| M-2 | Step names the per-branch send instant and baseline; does-not-claim states the one-convergence-read window | t16:194-199, t16:223-227 |
| M-3 | Does-not-claim: `CompletedAt` (API server clock) and `confirmedAt` (Hub clock) are two clocks; skew unmeasured; not new | t08:305-309 |
| M-4 | Does-not-claim: `CompletedAt` reads early for a backfilled row merged with a live one; gap unverified | t08:310-318 |
| M-4 | The same qualification in the mapper comment and the port field comment | t03:95-97, t01:201-204 |
| M-5 | Phase 3 t18 citations updated to :50-53, :65-68, :208, :218-223, :255-265, :269 | review:834-837 |

**Line numbers of round-1 rows that moved** (t16 grew by 3 lines at its head and 11 in the readiness
section): the Roll back gate is now t16:137, `roll back follows what is serving` t16:184, the edges
section t16:94-128, the three I7 tests t16:187-189, the step t16:194-199, does-not-claim
t16:213-227. t08's new tests run t08:252-256 and its does-not-claim t08:296-318.

**Deviation, declared.** M-2's fix said *"Have Push record the result of its `reload()`"*. Phase
3's `reload()` returns `void` (phase 3 t18:264), so Push cannot hold its result. The rule above
reaches the same outcome — the value `reload()` brings back becomes the baseline, with no second
call — through `Load.at` (phase 3 t18:231), which phase 3 already stamps.

**Mutants (plan text; none run, as no phase 7 code exists).** Named as most likely to survive:
1. **"Serves now" keyed on `row.deploymentId === env.current.deploymentId` alone.** It passes the
   target-row half of `rollback target keeps its drill-ins`. It is killed by the rollback-row half
   (`deploymentId: null`, `isCurrent: true`) and by `prod row keeps its drill-ins`.
2. **A baseline taken from whatever ready value is in the store when a load is sent.** It passes
   every edges assertion except the Push half: the pre-push value would be the baseline, and the
   `reload()` value's arrival would read as a move. The new Push clause of `readiness on the four
   edges` kills it.

Also named: sorting by `CompletedAt` but reading only the first `succeeded` row in served order
(killed by `TestCurrentOrdersByCompletion`), and an edge-4 load that clears the baseline instead of
recording the moved value, which would record the next move silently as a baseline (killed by the
two-successive-moves clause added to `readiness on the four edges`, t16:187).

**Left open after round 2.** M29 (t19) stays in its own batch. Nothing was compiled or run.

## Fix batch b05 — I2 and I6 (2026-09-24)

Both findings were re-checked against their sources, read only, before any edit. Both hold, and
I2 is wider than the review said.

- **I2.** `TriggerDeploy` (`handlers/projects.go:2942`) has **two** 402 writers. The first is the
  budget gate (`:2962` → `:2856`, *"new deploys are paused: "* + `budgetReason`,
  `handlers/budget_resolve.go:229-245`). The second is the risk gate (`:2979` → `:2916-2918`,
  *"production deploy blocked by the project's risk assessment: …"*). Both sentences are written for
  people and carry no `err.Error()`. The only other 402 writers are `handlers/web_search.go:604` and
  `:752`, on AI routes phase 7 never calls. `Rollback` runs neither gate (the only callers are
  `:1674`, `:1778`, `:2962` for the budget and `:1681`, `:1784`, `:2979` for the risk gate). The
  posture gate composed after the risk gate answers 403 (`project_security.go:155-157`).
- **I6.** Hub contract §7.4 `:711-715` (hub worktree) gives the Azure DevOps pattern
  `^[a-z0-9-]{1,50}\.visualstudio\.com/…`, compared *"byte for byte"*. Phase 3 task-13 `:140-143`
  (phase-3 worktree) spells `azure-devops/<org>/<project>/<repo>`. Phase 3 also disagrees with
  itself: its plan's H-3 row gives the Hub's form (cited as `:372` in round 1; the row had moved
  to `:378` by 918d356, so it is cited by anchor from round 2 on: the *"H-3"* row of
  `2026-09-23-phase-3-git-convergence.md`, feat/phase-3-git at 6a71014).
  The review's t06:106 is now t06:106-128, and t09:174-183 is now t09:211-227.

### I2 — a 402 is a refusal with its sentence

| Change | Where |
|---|---|
| Status-table row: 402 → `KindConflict`, `Message` = the sentence, with both gates cited | t02:115 |
| Test `TestPaymentRequiredIsConflict` (`Kind`, `Status 402`, `Message`) | t02:132 |
| `KindConflict`'s comment names 402 | t01:487 |
| Testdata `ship-402-budget.json` (the `:2856` + `budget_resolve.go:240` sentence) and `ship-402-risk.json` (the `:2916-2918` frame, stand-in reason) | t04:146-153 |
| Test `TestShip402IsARefusalWithSentence` (both files, one request each) | t04:177 |
| Ship step 6 *Refused*: the 402s arrive as `KindConflict`, abandon at once with Panacea's reason, and answer `409 deploy_conflict` | t09:214-217 |
| Route wire: `deploy_conflict` includes the 402s | t09:256 |
| Test `TestBudgetRefusalAbandonsWithSentence` (`Abandon` once with the sentence; no `Confirm`/`Track`; `409 deploy_conflict`; no `details.upstream`) | t09:273 |
| 7P-12 restates the row | plan:348-350 |
| §12: the 402 writers, settled from source; not observed live | plan:860-867 |

### I6 — HA7-6, and the cause is named until it is settled

| Change | Where |
|---|---|
| The false *"unaffected"* paragraph is withdrawn and replaced by **HA7-6** (blocking, to phase 3 and the Hub): both spellings cited; recommended answer is that phase 3's `RepoKey()` spells Azure DevOps as its own H-3 row and the Hub do; the Hub widening §7.4 is rejected (two spellings of one repository break §7.4's one-project-per-remote rule); the *meanwhile* | plan:678-706 |
| OD-4 row: nothing in phase 7 parses a host, the pattern only chooses a sentence, Azure DevOps Ship is blocked on HA7-6 | plan:121 |
| §10.1 phase 3 task 13 row names HA7-6 | plan:725 |
| §12: what HA7-6's evidence is (plan and contract text), and what was not checked | plan:868-873 |
| `Binding.HubMiss` (`""` / `not_linked` / `key_spelling`) | t06:58-62 |
| Step 3: `hubKey` sends `RepoKey()` verbatim (the swap point); `missCause` checks the Hub's published pattern, **after** a lookup that is always made | t06:114-128 |
| `HubMiss` is not on the binding wire; the desktop learns it through `releasesState` and readiness | t06:190-193 |
| **The binding test** `TestHubBindsADOWithTheHubsSpelling`: the Hub fake is seeded through `CreateProject` with the Hub's Azure DevOps spelling and uses the real `githost`; it binds, or it reads `key_spelling`, never `not_linked`; HA7-6 closes when it takes the bind branch | t06:209 |
| `TestHubMissNotLinked` (including direct `missCause` cases) and `TestHubKeyIsRepoKeyVerbatim` | t06:210-211 |
| Does-not-claim: Azure DevOps cannot Ship until HA7-6 is settled; the pattern is copied from contract text | t06:237-241 |
| Readiness precondition: `no_hub_project` splits by `HubMiss`; `key_spelling` gets a sentence naming the cause and `details.cause "key_spelling"` | t09:169-170, t09:108, t09:250 |
| Test `TestADOKeySpellingNamesTheCause` | t09:275 |
| `ReleasesState "hub_key_spelling"`, so the lens line agrees with the Ship bar | t07:116, t07:170-171, t08:35, t13:156, t14:67, t16:87 |
| Test `TestReleasesStateNamesKeySpelling` | t07:223 |
| The Hub-asks table gains the HA7-6 row, with which branch the binding test took | t20:54 |

### Deviations from the review's "how to fix", declared

- **`KindConflict`, not a new `KindRefused`.** `KindConflict` already means *"the target refused
  because of its current state"* (t01:487). A spent budget and a governed risk block are both
  states of the project. It already maps to `409 deploy_conflict` with the sentence (t05), it is in
  task 09's *Refused* list, and task 16 already renders a Ship `deploy_*` refusal verbatim as a
  `refused` history row. A new Kind would need a new problem code and a new desktop `ProblemCode`
  (task 13), with no behaviour different from this. `Status` stays 402, so a caller that later
  needs to tell the two apart can.
- **`ship-402-risk.json` was added beyond the review.** The risk gate is a second 402 writer on the
  same route, and before this fix it failed the same way.
- **The lens line was changed, beyond "readiness names the cause".** Without `hub_key_spelling`,
  task 14 would say *"not a shared project in the Hub yet"* above a Ship bar that names the
  spelling split. Two surfaces would disagree.
- **Phase 7 does not re-spell keys to make Azure DevOps Ship work now.** That would be a
  workaround in phase 7 for a defect in phase 3's task text, and a fourth canonicaliser that could
  drift. The root cause belongs to phase 3 (or the Hub), so it goes to them as HA7-6. `hubKey` is
  the one-line seam if they answer differently.
- **The binding test is green before and after HA7-6.** A test that stayed red until another phase
  acted would block every `go test` run in phase 7. Instead, phase 3's code fixes which branch the
  test takes, the test never allows the silent `not_linked`, and task 20 records the branch.

### Mutants

This batch changes plan text only. No phase 7 code exists, so no mutant was run. The two named as
most likely to survive once code exists:
1. **402 folded into a generic "other 4xx → `KindRejected`" row.** It keeps the sentence, and task
   09 still abandons (`KindRejected` is in the *Refused* list), so an abandon-only test passes. It
   is killed by the `Kind` equality in `TestPaymentRequiredIsConflict` and
   `TestShip402IsARefusalWithSentence`, and by the `409 deploy_conflict` in
   `TestBudgetRefusalAbandonsWithSentence`.
2. **`missCause` keyed on the host ("Azure DevOps → `key_spelling`") instead of the pattern.** Today
   every Azure DevOps key fails the pattern, so it passes the binding test and task 09's test. It is
   killed by the direct `missCause("kpmgcomau.visualstudio.com/…") == "not_linked"` case added to
   `TestHubMissNotLinked`.

Also named: a lookup skipped whenever the key fails the pattern (killed by
`TestHubKeyIsRepoKeyVerbatim`: the spy sees no Azure DevOps call), and a 402 given `Message ""`
by sharing the 5xx rule (killed by the `Message` equality in all three I2 tests).

### Left open

- **HA7-6 is open.** Until phase 3 or the Hub answers, Azure DevOps repositories cannot Ship from
  the cockpit (OD-4 is partly unmet), and the cause is named. This is recorded as an owner-visible
  blocker, not a decision for phase 7.
- Not checked: whether phase 3's *code* (as opposed to its task text) already spells the Hub's
  form; whether the Hub fake enforces §7.4 at `CreateProject`; how a `DefaultCollection` remote
  maps. Neither 402 was observed live.
- M26 (the ambiguous sentence after a reserve) touches the same abandon branches. It is left to its
  own batch.
- Nothing was compiled or run.

## Fix batch b05, round 2 (plan re-review b05-1: R-1 to R-6)

Date: 2026-09-24. Plan text only. The re-review (`.superpowers/sdd/p7/plan-rereview-b05-1.md`)
found I2 resolved and I6's mechanism sound, but HA7-6 one-sided. Round 1's *"Left open"* above is
superseded where this section says so.

**New evidence, read only:** the Hub **plan** (feat/hub at 52f0773), not only its contract, already
answers HA7-6. Its correction A1 replaces the contract's `.visualstudio.com` regex with phase 3 task
13's `^azure-devops/[a-z0-9-]{1,50}/[^/]{1,64}/[^/]{1,64}$`, citing phase 3's review I10; its task 01
publishes the rule as `hubapi.ValidRepoKey` and `HostOfRepoKey`; its task 04 step 2 has the fake
apply it at `CreateProject` (400 `bad_request`); its task 44a folds A1 into §7.4 before any code;
and it answered phase 4's HA4-6 *"= A1"*. This closes round 1's unverified *"whether the Hub fake
enforces §7.4 at `CreateProject`"* (from plan text; no Hub code read).

### R-1 — HA7-6 is a two-way decision, and the Hub's A1 already takes (b)

| Change | Where |
|---|---|
| HA7-6 rewritten: phase 3 task 13, its H-3 row and review I10, the Hub contract's §7.4 and H-3 (*"`githost.RepoKey` verbatim"*), and the Hub plan's A1, each cited by anchor with its commit; answers (a) and (b), (b) a **replacement**, not a widening; **recommended (b)** with its trade-off against (a); asks to phase 3 (fix I10 in the H-3 row, before editing it) and to the Hub (keep A1, fold it in 44a); what phase 7 changes under (a): only the two Azure DevOps literals in `TestHubMissNotLinked`; status: no longer blocking on the plans, open until the binding test passes against built code | plan:679-733 |
| §10.1 phase 3 task 13 row: *"must agree with"* the rule the Hub builds | plan:752 |
| §10.1 Hub H0 row: `hubapi.HostOfRepoKey` (Hub task 01, S0) | plan:745 |
| OD-4 row: phase 7 copies no key pattern; Azure DevOps Ship *"rests on"* HA7-6 | plan:122 |
| `missCause` asks `hubapi.HostOfRepoKey`; no pattern is copied; `hubKey` is unchanged under either answer | t06:118-139 |
| Binding test renamed `TestHubBindsADOWithPhase3sKey`: the seed is the real `githost`'s `RepoKey()`, never a literal; `CreateProject` must accept it and `Resolve` must bind | t06:221 |
| Does-not-claim: the plans agree, built code is unverified; the rule is imported | t06:249-256 |
| Task 20's HA7-6 row: the two asks, the renamed test, pass or red | t20:54 |

### R-2 — step 6 has a catch-all

| Change | Where |
|---|---|
| *Any other error* (task 02's 429 and *"any other"* rows, and a non-`*deploy.Error`) → `Abandon` with *"Panacea could not start the build."*; Panacea's API cannot answer 429 here (`routes.go:1286` is not under `middleware.Heavy`, `heavy.go:38`; Panacea at 8292f3f26), an edge 429 is unverified | t09:227-234 |
| Test `TestOtherErrorAbandons` (a 429 and a plain error) | t09:285 |
| The plan's Ship sequence says **any** Panacea failure abandons | plan:94-96 |

### R-3 — no sentence names a host

| Change | Where |
|---|---|
| Readiness `key_spelling` sentence: *"… the cockpit names it `<RepoKey>`, which is not a form the Hub accepts …"* | t09:170 |
| Lens line for `hub_key_spelling`, host-neutral | t14:67 |
| `missCause("github.com/dmackie1_kpmg/portal") == "key_spelling"` (GitHub `_` owner) | t06:222 |
| `TestADOKeySpellingNamesTheCause`: the sentence contains neither `Azure DevOps` nor `GitHub` | t09:286 |

### R-4 — three `HubMiss` states, three answers

| Change | Where |
|---|---|
| `HubMiss` comment: the three values; a Hub error keeps the previous `HubMiss` | t06:58-69 |
| Step 3: a Hub error keeps `HubProjectID` **and** `HubMiss`; a first-read failure stays `""` | t06:118-122 |
| `TestHubDownKeepsPanaceaBinding`: `key_spelling` survives a Hub failure; a first-read failure reads `""` | t06:219 |
| Readiness: `not_linked` row (was `!= "key_spelling"`); new `""` row, `details.cause "hub_unanswered"` | t09:169, t09:171, t09:108, t09:259-260 |
| Task 07 step 6: `""` with a configured Hub → `ReleasesState "unavailable"` | t07:170-173 |
| `TestReleasesStateNamesKeySpelling` gains the `""` case; `TestADOKeySpellingNamesTheCause` gains the `hub_unanswered` case | t07:225, t09:286 |

### R-5 — `HubMiss` is off the wire, enforced

| Change | Where |
|---|---|
| `HubMiss string \`json:"-"\`` | t06:68-69 |
| The wire paragraph names the tag and the test | t06:202-203 |
| `TestHubKeyIsRepoKeyVerbatim` asserts no `hubMiss` key (`strings.EqualFold`) in a marshalled `Binding` or the `PUT …/deploy/binding` 200 body | t06:223 |

### R-6 — citations into live sibling trees

| Change | Where |
|---|---|
| HA7-6 and §12 cite phase 3 and the Hub by anchor (*"H-3 row"*, *"`RepoKey` doc comment"*, *"correction A1"*) with the commit | plan:679-700, plan:895-903 |
| Round 1's `:372` citation annotated with its drift and the anchor | review:987-989 |

### Deviations from the re-review's "how to fix", declared

- **R-1: recommended (b), not (a), and no longer blocking.** The re-review asked to keep a
  recommendation *"if one is justified"*. The Hub's own plan has already adopted (b) (A1), phase 3's
  review recommends it (I10), and phase 3's code as specified already spells it. (a) would change
  the one component whose text is already right. HA7-6 stays open, but as an agreement to verify
  against built code, not a blocker on OD-4.
- **R-1: phase 7 changes almost nothing under (a), rather than "names what changes under (b)".**
  The re-review's list for (b) (`missCause`'s pattern, the binding test's seed, t09:170, t14:67)
  existed only because phase 7 copied one side's spelling. Importing `hubapi.HostOfRepoKey` and
  seeding the test with `githost`'s own output removes the copies, so neither answer changes those
  lines. Under (a), two literals in `TestHubMissNotLinked` flip, and the plan says so.
- **R-1: the binding test is now red on disagreement.** Round 1 made it green either way and
  logged the branch. With the plans agreeing, red is the true signal of an OD-4 break. It cannot
  block phase 7's `go test` early, because phase 7 cannot compile against `hubclient` before Hub S0
  (which contains tasks 01 and 04) lands.
- **R-4: `HubMiss ""` maps to `ReleasesState "unavailable"`** in task 07, beyond the re-review's
  readiness fix, so the lens and the Ship bar agree for a first-read failure too.

### Mutants

Plan text only; no phase 7 code exists, so none was run. The two named as most likely to survive
once code exists:
1. **`missCause` keyed on the host prefix** (`strings.HasPrefix(k, "azure-devops/")` → `key_spelling`).
   It passes every readiness test, which build the `Binding` by hand. Killed by
   `missCause("azure-devops/kpmgcomau/engagement portal/portal") == "not_linked"` and
   `missCause("github.com/dmackie1_kpmg/portal") == "key_spelling"` (t06:222).
2. **The catch-all abandoning with the error's own `Message`**, which is `""` for a 429 (task 02's
   row). It still abandons, so an abandon-only assertion passes. Killed by the exact message in
   `TestOtherErrorAbandons` (t09:285).

Also named: a Hub error resetting `HubMiss` (killed by `TestHubDownKeepsPanaceaBinding`'s two new
cases); a `json:"hubMiss"` tag (killed by the wire assertion in `TestHubKeyIsRepoKeyVerbatim`).

### Left open

- **HA7-6** stays open until `TestHubBindsADOWithPhase3sKey` passes against built code (task 20).
  Phase 3's I10 fix and Hub task 44a are text changes in their own trees; phase 7 does not make them.
- **Outside this batch:** the re-review's step-up finding (Panacea's F4 gate refuses any `pnc_`
  caller on a mapped production action, `stepup.go:208-214`). It belongs to the `pnc_`
  credential-handling lens, not to b05, and is recorded here so it is not lost.
- Unverified: whether phase 3's `ParseRemote` accepts a GitHub owner with `_`; whether either
  sibling's code exists or matches its text; an edge 429 or 405 on `POST …/deploy`; how a
  `DefaultCollection` remote maps.
- Nothing was compiled or run. No real Panacea or Hub was called.

---

## Fix batch b06 — I1 and I5 (2026-09-24)

Two Importants. Commits `f7a4a45` (I1) and `61bf3ca` (I5) on `feat/phase-7-deploy`, plus this
section's commit. Every Panacea citation below was read at `b33d15acb` with `git show`, read-only;
Panacea's HEAD is `f978765d1` and was not used.

**How each grep was checked.** A script pulled every fenced `go` block out of the part into one
file, with each line tagged by its part line, and the grep ran over that file. Each grep then ran
over planted violation lines, to show it still catches what it guards. Task 07's grep reads the real
desktop tree, so it ran over both this worktree and the main tree at `a92c3b0`.

### I1 — each acceptance grep prints what it promises

| Part | Was | Now | Run against the part's code |
|---|---|---|---|
| t01:639-647 | `-i`, 28 lines (the constants `"pending"`/`"succeeded"`, 25 Panacea-naming comments, `PosturePanaceaSSO`) | case-sensitive: `"/api/`, `"/projects`, `json:"`, `"(Succeeded\|Pending\|Failed\|Running\|Unknown)"`, `pnc_`, `/deploy/panacea"`. Expected: only `TestErrorNeverRendersCause`'s lines | the code prints nothing (exit 1). Five planted lines (a JSON tag, `== "Succeeded"`, a route constant, `"Bearer pnc_abc"`, a connector import) print one line each |
| t03:71, :361-364 | `// mirrors`, which printed 0, claimed "at least 17" | a count pair: `^// [a-z][A-Za-z]*DTO mirrors ` against `^type [a-z][A-Za-z]*DTO(\[T any\])? struct`, **17 and 17**. `branchEnvDTO`'s comment (t03:71) now opens *"branchEnvDTO mirrors"*, the only DTO that did not | 17, then 17. The review's "16, not 17" was true before b02 added `branchEnvDTO` |
| t04:209-214 | `Mode"\|ScanMode\|dastMode\|sastMode`, which printed the three `json:"accessMode"` lines | `-i` on the ten scan-mode spellings in `TriggerDeployOptions` (`models/models.go:2356-2372`). The `-i` is deliberate: `encoding/json` matches keys case-insensitively, so an untagged `DASTMode` field would still reach Panacea as `dastMode` | nothing. Planted `DASTMode string` and `` `json:"iacScanMode,omitempty"` `` print |
| t05:327-333 | bare `pnc_`, which also printed the `TokenSource` comment (t02:25) and panaceatest's `"pnc_test"` (`Server.Token`, t03:290, a non-test file) | `"pnc_"\|pnc_[0-9a-fA-F]{8}` (round 2 widened each quote to ``["`]``, M-6 below). Expected: exactly the prefix check in `deploy_connection.go`. The old text's *"the constant's comment"* named a constant no part defines | over the code of tasks 01–05, nothing. A planted `HasPrefix(token, "pnc_")` and a planted 32-hex key print. The refusal sentence *"starts with pnc_."* does not |
| t07:257-262 | bare `setInterval`, 3 lines (`heartbeat.ts:29` is `ReturnType<typeof setInterval>`) and a claimed `poller.ts:84` | `setInterval(` and `grep -v '\.test\.'`. Each expected line is quoted, and its number is given as of `a92c3b0` | exactly `api/poller.ts:90` and `hooks/heartbeat.ts:34`, in both trees |

Restatements fixed: `poller.ts:84` → `:90` at plan:81 and t07:6. Nothing else in the plan or the
parts restated the "17", the `// mirrors` form, *"only the comment on shipRequestDTO"* or
*"the constant's comment"*.

**Mutants.** The two expected to survive, named before running, and then run:
1. **t05: a key in upper-case hex.** It survived `[0-9a-f]{8}`. Panacea mints lower case
   (`hex.EncodeToString`, `handlers/api_keys.go:360-368`), but a hand-edited value need not be, so
   the class is now `[0-9a-fA-F]`. Re-run: the upper-case key prints, and the plan's own lines still
   do not.
2. **t04: a scan field under a name Panacea does not read** (`ScanModes map[string]string
   json:"scanModes"`). **It survives, and that is acceptable:** `TriggerDeploy` decodes leniently
   (t04:24), so the key is dropped and no scanner changes. It is recorded, not fixed.

Also run: `"SUCCEEDED"` survives t01's grep. It is not a word Panacea writes (t03:225-239).

### I5 — a key restricted to one project

**The facts, re-read at `b33d15acb`:**
- A restricted key's `/projects` holds its one project, applied before every other grant
  (`clients/kubernetes.go:1237-1249`).
- It is capped at Developer (`models/models.go:547-559`). `IsSuperAdmin`, `IsOwner` and `IsAdmin`
  are false for every scoped key (`:218-221`, `:247-249`, `:261-263`).
- `/me` says `role: "developer"` and `approved: true`, and its groups are the synthetic
  `project-<p>` or `project-<p>-viewer` (`middleware/api_key.go:380-385`). **No body phase 7
  reads says the key is scoped.**
- Refusals: an owner route answers `403 {"error":"forbidden: this token's scope does not permit
  owner access to this project"}`, as `text/plain` (`middleware/rbac.go:265-276`). A project outside
  the scope answers *"this token is scoped to a different project"* (`:194-202`).
- The portal's form: *Restrict to a project?*, which defaults to *No restriction — full access,
  same as you*. Once a project is picked, the capability defaults to **Read only**
  (`backstage/packages/app/src/plugins/AccountPage.tsx:1345-1355`, `:993-996`, `:1060`).

| Review's fix | Change |
|---|---|
| A recorded scoped-key `/me` and `/projects` pair | t03:265 `me-scoped-deploy.json` and `projects-scoped.json`. t03:266 adds `error-scope-owner-403.txt`, the probe's real refusal. t03:331 adds `TestScopedKeyIsOneApprovedProject` |
| The no-match sentence says "your key" | t06:154 now reads *"No Panacea project your key can see deploys this repository. A key restricted to one project sees only that project; otherwise, create one in the Panacea portal, then come back."* The code comment at t06:45 also changed. t06:218 adds `TestNoMatchUnderARestrictedKey` |
| The owner-only sentence says "your role or your key" | t18:111 now reads *"Only a project owner can change this, with a key that is not restricted to one project. Panacea refuses it to developers and to every restricted key."* t18:160's test asserts the line exactly. t07:238 `TestOwnerProbe` covers the scope `403` |
| The connect panel says which key to mint | t14:57: step 2 tells the user to leave *Restrict to a project?* on *No restriction*, in the portal's own words. A line under the steps says what a restricted key and a *Read only* key cannot do. The text is one constant, `KEY_ADVICE`, which is the seam. t14:84 adds `connect names the key` |
| An owner question | plan:789 **Q-P7-7**: an unrestricted key or one restricted at *Deploy*. **Recommended and built: unrestricted**, whose cost the question now states too: every right its holder has, platform admin included for an administrator, on every project (round 2, I-1 below). The plan's list of what the owner provides (plan:792) says so. plan:324 (7P-9) states the restricted-key case |

Also: t20:443 (LIVE-CHECK step 12) now connects once with a *Read only* restricted key, and checks
the three sentences live (OD-12, batched).

### Deviations from the review's "how to fix", declared

- **"Your key or your role" became two explicit halves.** The owner-only line names both *"a
  project owner"* and *"a key that is not restricted"*. The no-match line names *"your key"*, and
  what a restricted key sees. Each is one sentence that is true in both cases, because the watcher
  cannot tell the cases apart without matching Panacea's prose.
- **Added beyond the review:** the scope-`403` body, the task 06 test, the task 07 case and the
  LIVE-CHECK step. Without them, the new sentences would have no test and no live check.
- **Not built, recorded as a possible refinement:** detecting a scoped key. Panacea's
  account-surface routes refuse every scoped key with `403` through `DenyScopedToken`
  (`middleware/scope_guard.go:19-40`; `authedUnscoped`, `routes.go:3234-3243`, for example
  `GET /api/v1/me/hotbar` at `:3061`). A single probe could therefore say *"this key is restricted"*
  exactly. It was not built, for two reasons. It adds a route dependency on an account route that
  phase 7 has no other use for. And the answer to Q-P7-7 decides whether it is ever worth it.

### Left open

- **Q-P7-7** is the owner's.
- **The prototype still has the old owner line** (`prototype/phase-7-deploy.html:887`) and the old
  step 2 (`:813`). The prototype is the artifact the owner reviewed and was not edited. Task 14:57
  and t18:111 now differ from it on purpose, and the plan text wins.
- Unverified: the portal's rendered labels. Only the source was read, not a live portal. Whether a
  restricted key's `username` in `/me` is its owner's is also unverified, because the recorded body
  does not depend on it.
- Nothing was compiled or run. No real Panacea or Hub was called.

## Fix batch b06, round 2 — the re-review's I-1 and M-1 to M-6 (2026-09-24)

Input: `.superpowers/sdd/p7/plan-rereview-b06-1.md` (local, git-ignored). Panacea read at
`b33d15acb` with `git show`, read-only. Line numbers below are after this round's edits; the round-1
map above cites the lines as they stood at `e910fab` (its plan:789 and plan:792 are now plan:791
and plan:793).

| Finding | Change |
|---|---|
| **I-1** Q-P7-7 stated only the restricted key's costs | plan:791, the question column gains *"What the recommended key costs"*: an unrestricted key carries every right its holder has, on every project, platform admin included for an administrator (`IsSuperAdmin` reads the holder's own groups, `models/models.go:218-228`, re-read); phase 7 uses at most owner rights on the bound projects, and no route it calls needs admin. The per-project multi-key option is named as **not built** (the connection holds one key). The recommendation stands as built. review:1256 (the round-1 map's owner-question row) now states the cost too |
| **M-1** step 12 said "reconnect with your first key" | t20:443: reconnect with an unrestricted key, minting a new one if the first was not kept (`AccountPage.tsx:1406-1407`, shown once; t05:285 `TestConnectionGETNeverHasToken`), and do it **before** step 13, because `PUT …/auto-deploy` is `authedProjectOwner` (`routes/routes.go:1309`, re-read) |
| **M-2** "cannot Ship either" understated a *Read only* key | t14:57 `KEY_ADVICE`: *"… Restricted to Read only, it cannot Ship, roll back, rebuild, cancel or change who can open a branch."*, citing `routes/routes.go:1286`, `:1026`, `:1291`, `:1431` (each `authedProjectDeveloper`, re-read). t14:84's test asserts that phrase word for word. plan:326-328 (7P-9) and plan:791 (Q-P7-7) say the same |
| **M-3** t06's test named a wire fixture | t06:218: `deploytest.Fake` seeded with one project through `AddProject` (t01:538), in the shape `projects-scoped.json` records; the binding package loads no Panacea body |
| **M-4** 7P-6 quoted the old no-match sentence | plan:258-259 now quotes *"No Panacea project your key can see deploys this repository. …"* and points at t06:154 for the full sentence |
| **M-5** stale `t03:288` | t05:332 and review:1215 now cite `Server.Token`, t03:290, by anchor and line |
| **M-6** two greps blind to Go raw strings | t01:639 and t05:327 widen every quote class to ``["`]``; t01:645-647 and t05:334-336 say why. Planted and run below |

### M-6, run

Every fenced `go` block of tasks 01 to 05 was extracted (5, 2, 3, 1 and 5 blocks; scratchpad
`p7/b06r2/`) and each widened grep run over it, exit status read from a file, never through a pipe.

| Grep | Over the parts' own code | Planted survivors |
|---|---|---|
| t01:639, widened | t01's code: no output, rc=1 | `` `/api/v1/projects` ``, `` `/projects` ``, `` `Succeeded` `` and a raw-string import of `…/deploy/panacea` each print. The `"`-only form printed none of the four (rc=1) |
| t05:327, widened | tasks 01–05: no output, rc=1 | ``strings.HasPrefix(token, `pnc_`)`` prints; the `"`-only form missed it. `"pnc_"` and a 32-hex key still print. The comment naming `pnc_`, the refusal sentence and `"pnc_test"` still do not |

Named survivors before the run: (1) a raw-string `pnc_` check against t05's grep, (2) a raw-string
route constant against t01's. Both are now caught. The re-review's own planted survivors are the
same two.

### Deviations, declared

- **M-6 widened more than the review's example.** The example widened t01's two route quotes. This
  round widens the status-word quotes and the import quote too, since a raw string is equally legal
  there; each was planted and prints.
- **M-2 fixed in all three places**, not only `KEY_ADVICE`, because 7P-9 and Q-P7-7 restated the
  same understatement.

### Left open

- **Q-P7-7** is still the owner's. The question now shows both costs.
- Unverified: nothing was compiled; the greps ran over extracted plan code, not a built tree. The
  portal labels were read in source only. No real Panacea or Hub was called.

---

## Fix batch b07 — M1 to M6 (2026-09-24)

Minors in the domain, transport and read parts. Commits `88f3971` (M1), `863160f` (M2, M3),
`9060e27` (M4), `ac40a87` and `33022c7` (M5), `e04367a` (M6), `4732467` (named survivors), and
this section's commit (the map, and 7P-8's wording that the group's name stays `""`). Panacea
read at `b02424691` with `git show`, read-only. Line numbers are after this batch's edits. M1 to
M3's text was salvaged from an earlier b07 attempt that the watchdog stopped before it committed;
each salvaged line was re-checked against source before it was committed.

| Finding | Change |
|---|---|
| **M1** posture words are Panacea's; "exactly as `canonicalAccessMode`" | t01:396-401: the five words are adopted on purpose as the product's vocabulary (the control plane's API and the desktop type use the same five, t11:46, t13:179, both re-read), the one declared exception to t01:8's rule. t01:526-531: `ParsePostureMode` is *close to*, not the same as, `canonicalAccessMode`, which maps an unknown mode to `none` when `ssoEnforced == "false"` (`handlers/access_mode.go:75-77`, re-read); harmless because the GET already canonicalises (`:162`). t01:655 cites the constants at :405-409 |
| **M2** `BuildHTTPClient` must clone `DefaultTransport` | t02:76-86: `http.DefaultTransport.(*http.Transport).Clone()`, never `&http.Transport{}`; keeps `ProxyFromEnvironment`, `ForceAttemptHTTP2` and the idle and handshake timeouts; only `TLSClientConfig` is replaced; Go reads no Windows or PAC proxy. t02:153 `TestProxyFromEnvironmentHonoured` (a child process, because `ProxyFromEnvironment` reads the environment once; the recorder sees one `CONNECT panacea.invalid:443`), with t02:65 saying `Version` makes one attempt. t02:178 acceptance adds `Proxy`. t20:440 LIVE-CHECK step 1: a dial error where a browser reaches the address means set `HTTPS_PROXY` (t02:84-86) |
| **M3** file list; `Retry-After` on GET retries | The file list already names `cmd/versioncheck/main.go` (t02:15, added by b01 in `f75efdb`). t02:95-103: a 503 with `Retry-After` of at most 2 s waits `max(fixed delay, Retry-After)`; a longer one, and every 429, returns at once with `RetryAfter` set, so `do` never sleeps inside the watcher's cycle (the failure rule, t07:188-193, ends the cycle). t02:48 `Client` takes its sleep as `c.sleep`. t02:150 `TestGetRetryHonoursRetryAfter`; t02:178 acceptance adds `RetryAfter` |
| **M4** incoherent mutant; `b33d15acb` references | t03:341: the mutant is now *`Pending` read as terminal* (e.g. `BuildFailed`, the "not `True` is failure" reading of `derivePipelineStatus`, `clients/pipeline.go:2073-2084`, re-read), which stops `Track`'s 3 s cadence (t07:75-76) and lets `Advance` write a result (t01:562); the assertion adds "`Pending`'s state is non-terminal". t03:7 and t03:374 cite `406d0da95` (plan §2) and `b02424691`. Spot-checked unchanged at `b02424691`: `pipeline.go:2073-2084`, `handlers/security_status.go:192-201`, `:211-214` |
| **M5** empty environment; `prod` label; test-data names | **Empty group**: t08:161-175 rule 1b joins a `""` deployment by its run label, byte-exact, to a row's `branch`, which is Panacea's own attribution (`clients/kubernetes.go:5396-5415`, looked up at `:5455`, the row's `branch` from `e.Branch`, `handlers/projects.go:2118`). Otherwise one `unmatched` group, `name ""`, titled *"no environment recorded"* by the desktop (t08:43, t15:101-105, t15:203). t08:51 names rule 1b as the one join on a row's `branch`. t03:249-252 and t03:342: `SameEnvironment` does not adopt `tenantNamespace`'s `""` → base (`kubernetes.go:4213`); `("", "production")` is false. t17:131-132 and t17:138-139 give the drawer the sentence *"Panacea recorded no environment for this release."*; t17:242 tests it. t08:261 `TestNoEnvironmentJoinsByLabel`. Plan 7P-8 (plan:288-298) says the same. **`prod` settled**: plan:281-286 and plan:909-918 (moved from §12's unverified list to *Settled from source*). The API writes `"prod"` (`pipeline.go:237`, `:608`), a push writes `"prod"` (`webhook-receiver/models/event.go:116-129`, `pipeline/trigger.go:760`, `:1271`), and the advisory run's `TargetEnv` label (`pipeline.go:842`) never reaches the list (`kubernetes.go:5633-5642`, `isAdvisoryRun` `:140-142`). t20:156-158 and t20:190-197: the live test logs each environment label and the `""` count; t20:442 step 3 copies it. **Names**: already correct since b02 (`feat-evidence-upload`, 20 characters, not cut; `sanitizeBranchEnv`, `projects.go:3814-3826`, re-read). Every other sanitised name in the plan was re-derived and matches (`feat-phase-7-deploy`, `feature-evidence-upl`, `lane-7-deploy-connec`, `feat-x`) |
| **M6** `SetBranchAutoDeploy`'s response marked unverified | t04:132: verified, a `data` envelope with the GET's five keys, `enabled` a bool, the value just written (`projects.go:4192-4198`, `:4194`), the 409 at `:4178`. t04:50-52 `autoDeployDTO` names both handlers. t04:158-159 adds `autodeploy-set.json`; t04:183 `TestSetAutoDeployDecodesPut`. The step at t04:195-198 becomes a re-read. Added: t04:139 records `ClearBranchURLPrefix`'s body (`data {branch, routeReverted}` plus a `message`, `:4448-4451`), the other envelope that step asked for |

Citation moves: t05:332 cites `Server.Token` at t03:295.

### Deviations from the review's "how to fix", declared

- **M5, "join it by branch".** Read as joining by the **run label**, never a git branch, so that
  rule 1a stays true. The join copies Panacea's own index rather than deriving a label, so it is
  not a fourth copy of Panacea's rules. The group keeps `name ""` on the wire and is titled by the
  desktop. The alternative was a made-up name in the data, which could be mistaken for a real
  environment.
- **M5, `prod`.** The review cited `pipeline.go:196-206`, `:237`. At `b02424691` `:196-206` is
  the VCS input check; `:237` and `:2195-2207` are the facts. This batch also traced the two
  writers b02 left open (the push path and the advisory run's `TargetEnv`).
- **M3.** The file-list half was already fixed, by b01. Only the `Retry-After` half needed work.
- **M6, widened.** `ClearBranchURLPrefix`'s envelope, named in the same step, is recorded too.

### Mutants

These are plan edits, so no code exists to mutate. The two named survivors were checked against
the new tests' rows, by design, and not run:

1. A label join that case-folds (`strings.EqualFold`). None of `TestNoEnvironmentJoinsByLabel`'s
   first rows told it apart. It now has a `Feat-Evidence-Upload` row that must stay unmatched,
   because a Ship's label keeps capitals (`sanitizeLabelValue`, `pipeline.go:2351-2364`, re-read)
   and Panacea's map lookup is exact.
2. A hand-built transport that sets `Proxy` and `ForceAttemptHTTP2` but drops `DefaultTransport`'s
   idle and handshake defaults. `TestProxyFromEnvironmentHonoured` now compares
   `IdleConnTimeout`, `TLSHandshakeTimeout`, `MaxIdleConns` and `ExpectContinueTimeout` with
   `http.DefaultTransport`'s.

### Left open

- Nothing is compiled or run, and no real Panacea was called. The proxy test's child-process shape
  and the `CONNECT` count are from `net/http` knowledge, not a run.
- Whether the owner's network needs `HTTPS_PROXY` for Panacea is unverified. LIVE-CHECK step 1
  answers it.
- No owner question.

## Fix batch b07, round 2 — the re-review's I-1 and M-1 to M-5 (2026-09-24)

Source: `.superpowers/sdd/p7/plan-rereview-b07-1.md`. Panacea read at `b02424691` with `git show`,
read-only. Every Panacea line below was re-read today: `handlers/projects.go:1612` (Rollback passes
`targetDeploy.Environment`), `clients/kubernetes.go:5887` (`nsName := tenantNamespace(projectName,
targetEnv)`), `:4213` (`""` keeps the base namespace), `:5189-5201` (the production row's branch
is the newest prod run's), `webhook-receiver/models/event.go:117-118` (`"preview"` for PR events),
`handlers/logs_stream.go:100` and `logs_app_stream.go:115` (`Retry-After: 5` on a 429).

| Finding | Change |
|---|---|
| **I-1** a joined `""` row un-gates Roll back into production | t08:172-181 rule 1b: display and attribution only, no act reads it; the Panacea path to production cited. t16:137 the gate adds `environment !== ""`; t16:147-154 why, read before the lookup. t16:195 test `no rollback without a recorded environment`. t10:60-66 precondition before the reserve: `409 deploy_precondition` `no_environment`. t10:125-126 the wire. t10:156 `TestRollbackRefusesNoEnvironment` (no `Reserve`, no Panacea call). Restatements: plan:298-301 (7P-8), plan:922-924 (settled facts) |
| **M-1** t16's blank-name sentence | t16:163-166: a `""` row, matched or not, says *"Panacea recorded no environment for this release, so a rollback would land in production."*; the *"named `<row.environment>`"* line is for a non-empty name only. Pinned by t16:195 (*no text contains "named ``"*) |
| **M-2** `ParsePostureMode` comment vs its test | t01:531-532: *"It never maps an unknown or empty word to PosturePublic; only "none" does."* |
| **M-3** `Retry-After` "never overridden" | t02:97-105: honoured inside `do` up to 2 s; a longer one is returned and no caller waits on it yet; the cadence overrides it. The caller-side wait is its own open seam **S7-R1** (plan:791, t20:44), not M14 |
| **M-4** label join of every deployment survives | t08:267 `TestNoEnvironmentJoinsByLabel` gains a `prod` deployment labelled `develop` (joins `production`, not row `develop`) and a `preview` deployment labelled `develop` (own `unmatched` group) |
| **M-5** t11:46 → t11:45 | t01:398, t01:658. Line shifts from M-2 fixed too: t01:400 `(:656-658)`, t03:341 `t01:563` |

### Deviations, declared

- **The refusal code.** The review names "a 409 and a code". Task 10 already refuses a Rebuild with
  `409 deploy_precondition` and a `reason` (`no_exact_commit`, `no_branch`), so the rollback uses
  the same shape, reason `no_environment`, rather than a new top-level code.
- **M-3's open item** is a seam row (§10.2, S7-R1, restated in task 20's `SEAMS.md`), not an owner
  question: it is later work with a known fix, not a decision.
- **M-4's killing rows.** The review's row (a `prod` deployment labelled `develop`) is used, framed
  as an earlier production run of `develop`: Panacea's `production` row carries the newest prod
  run's branch (`kubernetes.go:5189-5201`), so an older one's label can equal another row's branch.
  A second row, `preview` labelled `develop`, kills the fallback variant (label join only when
  `SameEnvironment` finds nothing), which the first row does not.

### Named survivors (before the fix), and what now kills them

1. A control-plane refusal placed after the reserve (reserve, refuse, abandon). Killed by
   `TestRollbackRefusesNoEnvironment`'s *"the Hub fake saw no `Reserve`"*.
2. A label join used as a fallback for a non-empty environment with no row. Killed by t08:267's
   `preview` row.

Design check only; there is no code to run.

### Left open

- Nothing compiled or run; no real Panacea called.
- Whether a legacy `""` run with a live image exists on dev Panacea is unknown. I-1's severity
  does not depend on it.
- Whether task 12's log stream goes through `do` (S7-R1's reach) is unverified.
- No owner question.

## Fix batch b08 — M7 to M10, M26, M30 (2026-09-24)

Minors in governance (tasks 04, 11, 18) and the connection wire (tasks 05, 16). Commits `0a397c3`
(M7, M8), `10dd915` (M9, M10), `d45e606` (M26), `df8ab0e` (M30), `6428fb5` (named survivors), and
this section's commit. Panacea read at `b02424691` with `git show`, read-only; the Hub contract at
`a6ccd77`, read-only. Line numbers are after this batch's edits.

| Finding | Change |
|---|---|
| **M7** not-enforced message embeds `reconcileErr` | Re-read: `project_security.go:818-821` sets `reconcileErr = err.Error()`, `:837-839` puts it inside `(…)`. t04:85-88 the DTO comment; t04:139 `SetPosture` carries Panacea's `message` only when `enforced` is true, and otherwise writes **our** sentence from the structured `enforced` and the mode and branch it sent; Panacea's sentence is logged at warn. t04:165-167 the fixture's branch is `feat/fix(ui)` and its stand-in error holds a `(`. t04:192 `TestPostureNotEnforced` asserts equality with our sentence, and that neither the address nor `retry` leaks. t01:442-444 `PostureResult.Message`; t11:118; t18:131-133; plan 7P-12 (plan:375-378) says a `200` sentence that embeds an `err.Error()` is not carried either |
| **M8** the parsed 500 sentence | Plan 7P-12 (plan:382-390) gains *"No Panacea sentence is ever parsed, with one exception"*, naming `Grant`'s 500 read, why it is kept (it fails safe to `KindUnavailable`), and its pin. t11:101-105 names it as the single exception; t04:196 points there. Source re-read: `handlers/access.go:57`, `clients/keycloak.go:182`, `:189` |
| **M9** one status per code | Nil `Deps.DeployConnection` → **409** `deploy_not_connected`, the code's declared status (t05:346, and the wire at t05:180-181). A locked origin against another → `ErrOriginLocked`, mapped by the `PUT` to **400** `bad_request` before `writeDeployError` (t05:108-110), listed in the wire with its sentence (t05:166-168). t05:304 `TestLockedOriginRefusesAnother` (the sentinel, nothing stored, no request, an equal origin accepted); t05:305 `TestNilConnectionIs409` |
| **M10** origin row missing from Hub §9.1 | Took the review's second option. Hub contract `a6ccd77` §9.1 (`:1165-1175`) lists only the token row. The origin moves to `statepath.Dir("deploy")/connection.json` (t05:42-50, `Config.StateDir` t05:75-78, steps 4 and `Disconnect`, t05:348 wiring). t05:302 `TestOriginWrittenAfterToken` asserts one `Put`, to the token row. The origin file is the first phase 7 file written, so ruling R2's `writeAtomic` lift moves from task 06 to task 05: t05:24, t05:333-337, t06:20, t06:228-230, plan:541-543, plan:781. Plan 7P-13 (plan:393-396) names the file |
| **M26** "not started" after a timeout | Panacea's `upstream_unavailable` carries `details.status`, a JSON number, `0` when there was no HTTP answer (t05:263-267, t05:277-282; plan 7P-12 plan:368-370). `writeDeployError` gains `extra` details, and Ship's step 6 passes `reserved: true` (t09:237-240; wire t09:271-275). The desktop picks one of four sentences from `reserved` and `upstreamNoAnswer` (t16:177), with *"may still have started the build"* for reserved + status 0. `upstreamNoAnswer` is t13:294-297. Tests: t05:313, t09:292, t13:345, t16:188 |
| **M30** revoke by grantee; role change; legacy row | Re-read `clients/keycloak.go` at `b02424691`. `Revoke` takes the row's `group` everywhere: t01:122-125, t04:142, t11:77-82 (`DELETE ?group=`, 400 without it), t13:241, t18:51, t18:81, t18:147-149. A role change is **one** `Grant`: t01:119-121, t04:141, t11:66-70, t18:102, t18:144-147. Legacy rows: t01:462-475 (`Legacy`), t04:97-100, t04:140, t04:167-169 fixture, t11:56-61, t13:186-189, t18:139-142, plan B1 (plan:134). Tests: t04:198-199, t11:122-123, t18:175-177, t13:357 |

### Deviations from the review's "how to fix", declared

- **M7, "trim after the first `(`".** Refused. The branch comes before the `(` in Panacea's format
  string, and git allows `(` in a branch name (`git check-ref-format --branch "feat/fix(ui)"`
  exits 0, run here). A cut there also drops the sentence's fixed tail. Keying on `enforced` and
  writing our own sentence parses nothing.
- **M30, "a held Revoke followed by a Grant".** Wrong on the facts at `b02424691`.
  `AddProjectAccess` is single-role: it removes the grantee from the project's other role groups
  before adding the new one (`keycloak.go:172-173`, `:198-214`). So the old role is not left in
  place. A Revoke then a Grant would leave the person with no access if the Grant failed. A role
  change is one `Grant`. The drop is best-effort (`:207-210`), so the re-read is drawn as it is,
  and a grantee on two rows is shown on both.
- **M30, "a legacy row (empty `grantee`)".** The empty grantee is also wrong on the facts.
  `ListProjectAccess` fills a legacy row's `grantee` from its `panacea-group` attribute, and it
  skips a member-less group with no attribute (`keycloak.go:143-153`). Panacea's own test pins
  `grantee "team-old"` (`clients/keycloak_access_test.go:349-351`). Only the model's comment
  (`models/models.go:2011`) says `""`. The plan's B1 row and tasks 01, 04, 11, 13 and 18 said
  `""`. They now mark a legacy row as `group != grantee`. A person's row has `group == grantee`
  (`:156-159`), which also made t11's old example (a person with a group name) wrong.
- **M26, widened.** Keying on `status` alone would tell a Ship that failed at step 4 (before any
  reserve) that a build may have started. So step 6 also marks `reserved`. Any other act with
  status `0` gets its own "cannot tell whether it acted" sentence.
- **M10.** Took the review's second option instead of adding a Hub row, because the Hub contract
  is the Hub stream's file and read-only here. The origin is also not a secret. And a
  `PanaceaApp/panacea/current` row would share the `<api host>` namespace with a token row.

### Mutants

These are plan edits, so no code exists to mutate. The two named survivors were checked against
the new tests' rows, by design, and not run. Each gained a row:

1. The desktop's `revoke` keeping the old `?grantee=` parameter name with the group's value. Only
   task 11's route would refuse it, and no desktop test saw the URL. t13:357 `revoke sends the
   group`.
2. `connection.New` failing startup, or deleting the token, on a torn `connection.json`.
   t05:303 `TestOriginFileMalformedIsAbsent`.

Checked, already killed by this batch's rows: `upstreamNoAnswer` written with `==` (the `"0"`
case, t13:345); Ship marking `reserved` on step 4's failure (t09:292); `Legacy` computed as
`Grantee == ""` (t04:198); the not-enforced cut at the first `(` (t04:192's equality).

### Left open

- Nothing compiled or run; no real Panacea called.
- Observed, not a finding of this review: Panacea's own create timeout answers **500**
  *"failed to start deployment: context deadline exceeded"* (`ship-500-timeout.json`). The
  PipelineRun may still have been created, but t09 step 6 calls a 5xx *"could not start"*. The
  desktop now says *"not started"* for it (M26 case b). Whether Kubernetes can create the run
  after Panacea's 15 s context expires is unverified. The review should rule on it.
- Observed: t18:102 justifies sending a grant at once with *"none opens the app to anyone new"*,
  yet a grant does exactly that for its grantee. This is outside this batch's findings, so it is
  left as found.
- Whether dev Panacea (v1.163.0) behaves as `b02424691` does for the legacy row's `grantee` is
  unverified. LIVE-CHECK does not exercise a legacy binding.
- No owner question.

### Fix batch b08, round 2 — the re-review's open issues (2026-09-24)

Fixes `.superpowers/sdd/p7/plan-rereview-b08-1.md`. Commits `2df4190`, `1e7190e`, `d97244d`
and this section. Panacea read at `b02424691` with `git show`, read-only. Nothing compiled or run;
no Panacea environment called. Paths are under `docs/superpowers/plans/`; `tNN` is
`2026-09-23-phase-7-deploy-parts/task-NN.md`, `plan` is `2026-09-23-phase-7-deploy.md`.

| Issue | Change (file:line) |
|---|---|
| I-1 a demotion sent at once; "never strands" false | Verified `keycloak.go:198-218` (drop, then add) and `access.go:55-58` (a failed add is a 500). **t18:106** a lowering is held, with its consequence first. **t18:108** the "sent at once" row now reads "add a person, raise a role": none takes access away when it succeeds, and a failed raise is re-read. **t18:149-171** Add at a current member is that row's Change role; the only-owner guard (disabled, with a sentence) and the "This is you" line from `self` (t18:71, :84, from task 13's `ConnectionStatus.account`). **t18:56-61** `useGovernance` re-reads access after any failed grant or revoke (a revoke also stops at its first failed group, `keycloak.go:253-256`). Tests at **t18:201-204**. Rewrites: **t11:66-73**, **t11:129**, **t01:119-122**, **t04:150**, **plan:134**. Unverified, stated at **t18:232-241**: that `/me`'s `username` is the Keycloak username the list shows (`keycloak.go:523-528`), and that a demotion's 403 is immediate for a `pnc_` key |
| I-2 Rollback and Rebuild reserve too; a 5xx called "not reached" | **t09:30-33** declares `ship.ErrAfterReserve`; **t09:252-256** wraps every step-6 error with it (`%w: %w`, Go 1.23), and the route passes `reserved: true` only when `errors.Is`. **t10:31-67** `withReservation` carries the wrap and takes `AbandonWords`: `BuildWords` for Ship and Rebuild, `RollbackWords` for Rollback (**t10:96**, **:127**). **t05:269-279** the server sentence follows five classes of `Status`; **t05:292-297** names all three reserving routes. **t13:294-303** `upstreamAnswer` replaces `upstreamNoAnswer` with the same classes. **t16:177** the banner keys on `reserved`, then the act, then the class; a 5xx is never "nothing was sent". **plan:368-374** restates it. Tests: **t10:186** `TestRerunErrorsAreReserved` (beside t09's `TestTimeoutAbandonsAmbiguously`), **t09:312**, **t05:328**, **t13:351**, **t16:188**. A gateway's 502 or 504 is classed "no answer": a proxy that gave up on Panacea says nothing about whether Panacea acted — declared, root-cause option |
| M-1 the pin's fixture was an outage | **t04:206** one fixture per real phrase (`:189`, `:182`), plus `:186` and `:177` outages asserted `KindUnavailable`. The matcher is anchored: the text after the prefix **begins with** the `:189` phrase or **equals** the `:182` one, never a contains. Restated at **plan:387-394**, **t11:98-112**, **t04:150** |
| M-2 "empty grantee" | **plan:387-389**, **t11:100-102**: an empty grantee is a 400 first (`access.go:50-52`, verified); only a whitespace-only one reaches `:182`, and task 11 refuses it before sending |
| M-3 nil answer tells a connect to connect | **t05:98-101** `New`'s error contract; **t05:365-371** a `New` error is fatal at startup (`serve`, `cmd/api/main.go:66`, returns it; `run`, `:47`, exits 1 — verified in this tree), with a `main_test.go` row; **t05:185** nil occurs only in tests; **t05:23** adds `main_test.go` |
| M-4 built errors carry no status | **t01:518-522** Status is the status received, a 2xx for a built error. **t04:142** run-less 202 → `Status 202`; **t04:144** rollback id mismatch → `Status 200`; **t03:217** and **:344** empty `env` → `Status 200`. **t09:238-241** a run-less 202 abandons with its own sentence, not "could not start". Test **t04:198** |
| M-5 `notEnforcedSentence` undeclared | **t04:97-104** declared, with the wire word and plain text and the exact example; **t04:148** points to it; **t04:174** the fixture's `accessMode`; **t04:202** asserts the example byte for byte |
| Ruling, Ship 500 | **t09:224-231** keeps "could not start the build" for Panacea's own 500, cites `pipeline.go:687-690`, `:710-716` and `projects.go:2985-2991` (verified), and names the one remainder |
| O-1 per-attempt timeout under Panacea's worst case | **Not fixed here.** Outside this batch, and Important for the main review: it changes t02's timeout, which other batches own. Handed back to the orchestrator |
| O-2 `projectAccessGroups` prefix reaches `app-two` | Verified `keycloak.go:483-490`. A Panacea defect; recorded at **t18:235-239** and as an owner question. The cockpit cannot filter it: a person row's `group` is the person |

The fixer's round-1 note on t18:102 is closed by I-1: the row no longer claims "none opens the app
to anyone new".

Survivors I expect a re-review to name: (1) `upstreamAnswer`'s "refused" class for a 3xx, which
says "nothing was changed" of a sign-in redirect, true only if no edge acts on a POST before
redirecting; (2) the only-owner count, which skips legacy owner rows on the strength of the
comment at `keycloak.go:166-170` rather than a test of Panacea's RBAC.

## Fix batch b09 — M11 to M16 (2026-09-24)

Minors in the binding (task 06) and the watcher (task 07), plus the plan's §10.1 row. Commits
`e5327e4` (M14), `070a354` (M13), `b86dc24` (M11), `42b9298` (M12), `431b810` (M16), `75edd95`
(M15), `c5e941a` (M11 follow-up), and this section's commit. The Hub contract was read at
`6b873d0` in the hub worktree, read-only (the review's §9.3:1368-1376 is now `:1377-1385`; §8's
release rows are `:1155-1159`; the list rule is §1 `:48-49`). The review's own anchors had moved:
t06:86-89 is now t06:100-112, t06:95-99 is t06:117-121, plan:666 is plan:797. Line numbers below
are after this batch's edits.

| Finding | Change |
|---|---|
| **M11** a pin beyond the 200th release reads "no brief is recorded" | Both of the review's options. **t07:132-139** `Snapshot` gains `ReleasesMore` (the page's `next` was set), `ReleasesAt`, `PinsByRun`. **t07:224-244** step 6: a good read sets them; while `ReleasesMore`, each unmatched history row gets one `Releases(…, {PipelineRun: d.ID, State: "confirmed", Limit: 1})`, at most 20 per project per cycle, cached; `pipelineRun=` not `deploymentId=` because only the reserver's watcher writes the join; the reads run **last** in the cycle (after every project's steps 1-9), and a deadline cut there is no failure. **t07:246** the join also covers `PinsByRun`. **t08:108** `ReleaseRow.releaseLoaded`; **t08:195** rule 5 searches the page, then `PinsByRun`; **t08:205-213** rule 5b says when a null pin is "not loaded" (page truncated and not yet read; or no release read has succeeded yet, or `hub_key_spelling`) and when it is an answer. Tests **t07:351** `TestPinsBeyondThePage`, **t08:284** `TestPinNotLoadedIsNotAbsent`. Restated: **t13:135** the type, **t15:131** the pin cell *"pin not loaded"*, **t15:146-148** and **:201** the origin *"not loaded yet"*, **t16:159-163** and **:201** Rebuild disabled with its own sentence, **t17:145**, **:164**, **:232** the drawer |
| **M12** `release.*` feed events do not reach `Watcher.Nudge` | **t07:88-96** `Watcher.HubEvents(events)`: a `release.*` event whose `projectId` is a bound project's `HubProjectID` marks its releases due and Nudges it; no request. **t07:216-218** step 6 reads releases on such an event. **t07:307-321** the wiring: phase 7 adds no second feed reader; `main.go` hands `HubEvents` to C2's poller, or this task adds an `OnEvents` listener field to it as a declared amendment (HA7-4's pattern), or, before C2, releases stay on the 60 s cadence. **t07:18-19** file list; **t07:360-363** the step that finds C2's poller. **plan:792** the Hub C2 row. Test **t07:352** `TestReleaseEventRefreshesReleases` |
| **M13** `KindNotFound` / `KindForbidden` from `Environments` unspecified | **t07:166-182** step 2: any `Environments` kind but `KindUnavailable` and `KindUnauthorized` is about the binding: other projects are still read; this one goes `Stale` with Panacea's sentence, `FetchedAt` unchanged, is Nudged, and is re-resolved next cycle. Not bound → its data is dropped; bound elsewhere → reset. **t07:155-162** step 1: a changed `Binding.Panacea` resets the snapshot's Panacea and Hub data, so one project's rows never appear under another's binding. **t07:264-266** the failure rule says so. **t08:33-35** `staleReason` gains `deploy_not_found`, `deploy_forbidden`, `deploy_conflict`, `deploy_rejected` (t05:281-284's codes); **t15:152-158** the stale banner says *"Panacea refused to show this project"*, never *"has not answered"*. Tests **t07:345** `TestEnvironmentsRefusedReResolves`, **t15:212** `a refusal is not silence` |
| **M14** one hung read blocks the loop about 46 s | **t07:62-65** `Config.CycleDeadline` (zero = 15 s); **t07:76** `Run` says each cycle runs under it. **t07:268-285** the per-cycle deadline: one `context.WithTimeout` over the whole cycle; expiry is the cycle's `KindUnavailable` (Status 0), so the failure rule applies unchanged; tracked-run projects first, the rest in rotation, so a cut cycle starves none. **plan:523** `PANACEA_APP_DEPLOY_CYCLE_DEADLINE`; **t07:365-366** `config.go`. Test **t07:346** `TestCycleDeadlineCutsAHungRead`. **t02:101** now cites the failure rule at t07:259-266 |
| **M15** plan's by-name fallback does not exist | **Corrected the row, not built** (below). **plan:797**: phase 3 task 13 is a hard dependency; 7a builds 01-05 and waits. **t06:3-6** the part's header says the same |
| **M16** one `Deployer.Projects` per local project per cycle | **t06:100-112** `Resolve` is `ResolveMany` of one id; `ResolveMany` makes one `LocalProjects` read, one `Deployer.Projects` read and one Hub lookup per distinct key for every id. **t07:40-44** the watcher's `Bindings` interface takes `ResolveMany`; **t07:155-160** step 1 calls it once per cycle for every due project, and not at all when none is due. Tests **t06:231** `TestResolveManyReadsProjectsOnce`, **t07:332** `TestOneProjectsReadPerCycle` |

### Deviations, declared

- **M11: both options, and one wider case.** The targeted read alone leaves rows blank while it
  runs, and "pin not loaded" alone leaves old rows unread for good, so the plan does both. The
  same inversion arises when the Hub has never answered for this binding (down or signed out at
  the first read) and under `hub_key_spelling`. This is the same defect, not a tangent, so rule
  5b covers those cases too.
- **M12: a listener, not a second reader.** The review says "have the Hub change poller nudge
  the watcher". The poller is C2's code, so the plan wires into it, as HA7-4 does for the cursor.
  A second `/v1/changes` reader would double the feed load.
- **M13: the rule is total.** The review names two kinds. The plan covers every kind except the
  failure rule's two, so no `Environments` answer is left unspecified. The stale banner changes
  with it, because *"has not answered"* is false for a refusal. The desktop type is already
  `code: string` (t13:156), so no type changes.
- **M14: 15 s, not 3 s.** *(Superseded by round 2, I-1 below: the budget is now 30 s, and its
  expiry is a budget stop, not a `KindUnavailable`.)* A deadline at the active cadence would cut every normal `Deployments`
  read: that read makes a dozen PipelineRun reads inside Panacea (`projects.go:1227-1253`). 15 s
  is one per-attempt timeout, so a hang costs one attempt instead of three. The ordering rule
  gives the tracked run the first share of the cycle.
- **M15: the row is corrected, and no fallback is built.** Four reasons:
  1. Part 06 imports `githost`, so there is no runtime state "without githost" in which a screen
     could ask anything.
  2. A by-name binding needs a key for the choice file and the Hub lookup. Without `githost`,
     phase 7 would have to spell that key itself, and t06:133-136 forbids a fourth canonicaliser.
  3. `Choose` takes only candidates on purpose (`TestChooseOutsideCandidates`). A name with no
     repository match could point Ship at a project that builds another repository, which would
     pin a brief to the wrong code. That breaks the provenance thread.
  4. The one real "no match information" case, no `repoUrl` from Panacea, already offers every
     project (`NoRepoInfo`).
  Verified: no `control-plane/internal/githost` exists in this worktree or in the phase-3
  worktree.

### Mutants

No code changed in this batch, so no mutant was run. Each new test names the mutant it kills.
Survivors I expect a re-review to name:
1. A slow Hub **page** read (step 6, once per project, inside the project's own steps) can still
   use up `CycleDeadline`. The next project's Panacea read then fails on a finished context, and
   the failure rule stales every project as if Panacea were down. Only the targeted reads moved
   to the end of the cycle.
2. A `nil` in `PinsByRun` is cached for the binding's lifetime. A release confirmed for an
   off-page deployment after its targeted read would never be seen. The plan argues this cannot
   happen, because a new deployment's release is always on the first page, but no test pins that
   argument.

### Unverified

- The Go names `hubapi.Page.Next`, `ReleaseQuery.PipelineRun` and `.State`, and
  `hubapi.Event.Kind` and `.ProjectID`. They are taken from the wire shape, and t07's "does NOT
  claim" section says so.
- C2's poller: its file, and whether it has a listener. No Hub code was read.
- That the Hub fake honours `pipelineRun=` and pages at 200. `TestPinsBeyondThePage` stays red
  until it does.

### Fix batch b09, round 2 — the re-review's open issues (2026-09-24)

Source: `.superpowers/sdd/p7/plan-rereview-b09-1.md` (I-1, M-1 to M-5). Commits `7c59ba6` (I-1,
and M-1's step 6 text), `16a8968` (M-3, M-4), `a569f6a` (M-1 helper and test, M-2, M-5), and the
commit that carries this section (I-1 phase alternation, this map). Line numbers are at that
last commit.

| Finding | Change |
|---|---|
| I-1 a cycle out of budget read as "Panacea is down" | **t07:317-366** "The cycle budget", three rules: (1) a Panacea read begins only with `ReadTimeout` left (t07:324), so a read that fails is Panacea's and the failure rule holds; (2) each Hub read under its own `HubReadTimeout`, so a dead Hub is still `"unavailable"`; (3) a **budget stop** (t07:335) stales nothing, polled projects keep fresh data, the stopped project keeps its snapshot whole, one Info log line. **t07:345-360** order: a Panacea phase, then a Hub phase (step 6's page reads moved there, t07:251-252 — the review's fix 4), then step 9; after a stop the phase that ran second goes first, so neither starves. **t07:62-70** `ReadTimeout`, `HubReadTimeout`, `CycleDeadline` zero = `ReadTimeout + Interval` (30 s), `New` refuses less than `ReadTimeout + ActiveInterval` (fix 3). **t07:305-315** failure rule ends the Panacea phase only, and never on a budget stop. Tests **t07:427** `TestCycleDeadlineCutsAHungRead` (now with `ReadTimeout` and the `New` refusal) and **t07:428** `TestBudgetStopIsNotAnOutage` (fix 5, plus a Hub-starvation case). Config **t07:448-454**; unverified **t07:484**. Restated: **plan:523** (fix 6), and the name "per-cycle deadline" at **t02:105**, **t20:44**, **plan:815** |
| M-1 a cached `nil` re-creates the inversion | Took neither of the review's two options as written. Option 1 (older than the page's oldest `reservedAt`) can stay *"not loaded"* for good on a project that stops shipping; option 2 re-reads every `nil` on every event. Instead, **settled**: a targeted read waits until `d.CreatedAt` is `pinSettle` before now, `pinSettle` = the page's longest `expiresAt − reservedAt` (the Hub's reservation TTL, contract `:1021` at 6b873d0, verified) + 5 min skew. After that a confirm is refused (`:1038`, verified), so the `nil` is final. **t07:274-284** the rule; **t07:140-147** the comment corrected (the false justification removed); **t07:156-161** `Snapshot.SettledBy`, one helper for the watcher and the view. Test **t07:434** `TestLateConfirmedPinIsNotCachedAbsent` (the review's test); **t07:433** `TestPinsBeyondThePage` seeds settled rows |
| M-2 rule 5b listed unknowns, and one sentence for three causes | **t08:215-244** rewritten as a list of answers: `no_hub`/`no_hub_project`; a `PinsByRun` entry; a complete page that answers for the row (settled when read, or read after the `Deployments` list with no open reservation). Everything else, including a zero `ReleasesState`, is not loaded; a zero state goes on the wire as `"unavailable"`, so the enum keeps six values and `types.ts` is untouched. Found in passing and closed in the same rule: a row whose release is still `reserved`, and a row newer than the last good page, were also drawn *"no brief is recorded"*. **Why not loaded** (t08:241-244) keys the sentence to `releasesState`: **t17:145** drawer (sign in; the HA7-6 key-spelling cause, no promise), **t16:161-166** Rebuild reason, **t15:131** pin cell. Tests **t08:315**, **t17:232**, **t16:204** |
| M-3 a refusal for good re-resolved every 3 s | **t07:205-219** only a **new** refusal (not already in `LastError` for this binding) marks and Nudges; a repeated one waits for the recheck. **t07:313-315** failure rule restated. Test **t07:426** gains the 8-tick case, at most 2 `Projects` reads |
| M-4 "cycle" meant two intervals; three by-name sentences | **t07:164-176** Terms: cycle = one 3 s tick, poll = a project's due read, recheck = `4 × Interval` measured in time. Every "every fourth cycle" in t07 is now "at its recheck (60 s)" (t07:183, :186, :214, :219, :225), steps 4-5 say "poll" (t07:221, :232, :239, :246); the HubEvents comment (t07:99). Tests restated in seconds with a tracked-run variant: **t07:413**, **t07:415**, **t07:416**. Restated: **plan:250**, **plan:520-521**, **t15:87**. The by-name fallback: **plan:874**, **t03:378**, **t20:145** now name task 06's `NoRepoInfo` (t06:154, verified) |
| M-5 t08 had no rule for `staleReason` | **t08:147-155** rule 0: `staleReason` is `null` exactly when `stale` is false, else `writeDeployError`'s code and message for `LastError` (t05:268-286, verified). **t08:31-36** the `fetchedAt`/`stale` comments reworded; **t07:150-152** the `Snapshot` comments likewise. Test **t08:316** `TestStaleReasonIsWriteDeployErrors` |

#### Deviations, declared

- **I-1, the budget.** 30 s is `ReadTimeout + Interval`, not the review's example `ReadTimeout +
  ActiveInterval` (18 s). At 18 s a read could begin only in the first 3 s of a cycle, so a normal
  multi-project cycle would stop almost every time. 30 s gives a 15 s window to begin in. The
  floor `New` enforces is the review's 18 s.
- **I-1, the Hub phase runs after a Panacea failure.** The failure rule used to end the whole
  cycle. Hub reads do not depend on Panacea's answer, and a Panacea outage should not freeze a
  teammate's rollback on the lens, so only the Panacea phase ends.
- **M-1**, as above: neither option as written, for the reasons in its row.

#### Mutants

No code changed, so no mutant was run. The two I expected to survive, named before the
self-check:

1. **The Hub phase starving behind a Panacea phase that finishes with less than `HubReadTimeout`
   left.** Found real in the draft: its exception covered only a stop in the Panacea phase. Fixed
   at t07:354-360 (the phase that ran second goes first after any stop), with a test case at
   t07:428.
2. **`SettledBy` trusting `Deployment.CreatedAt`.** Panacea keeps the earliest `CreatedAt` when it
   merges a backfilled row (t01:209-210), so a live row merged with a backfill could read as
   settled early and get a targeted read while its release is still `reserved`. Open: this needs
   a live row and a backfill of the same run, which is unverified as reachable. If it is, key the
   settle rule on the watcher's first sight of the deployment, not on `CreatedAt`.

#### Unverified

- `hubapi.Release`'s `ReservedAt`/`ExpiresAt` Go names, and whether `hubclient` has a timeout of
  its own. The watcher applies `HubReadTimeout` itself, so it does not depend on one (t07:480-483).
- That 30 s suits the owner's team. A stop stales nothing, so a wrong value costs latency; the
  Info log line on each stop makes it visible (t07:484).
- The re-review's own open items (C2's poller shape; the Go names; the `Deployments` read time)
  are unchanged by this batch.

## Fix batch b10 — M17 to M22 (2026-09-24)

Minors in the view (task 08) and Ship (task 09), and count fixes across tasks 12, 13 and 15.
Commits `132b98d` (M18, M19), `6825181` (M17), `907796d` (M20, M21, M22), and the commit that
carries this section. The Hub contract was read at `6b873d0` (its last change; the hub worktree is
at `af72f19`), read-only. Panacea was read at `~/Projects/Panacea`, read-only. Line numbers are
after this batch's edits.

| Finding | Change |
|---|---|
| **M17** an unshared repository turns Ship off with no way forward | The fix is the Hub's own share, not a phase 7 form. **t14:66** the `no_hub_project` line gains **Share with a team…**, which calls the store's `openShare(projectId)` (Hub task 42's `SharePopover`, `task-42.md:43-46`, verified). That popover posts `POST /v1/projects/{projectId}/hub-link` (Hub task 35, seam S-LINK, verified), which calls `CreateProject` (contract `:705`, verified). Without task 42, no button is drawn and the sentence ends with a true line. **t09:238** the precondition sentence names the share. **t07:100-106** `HubEvents`: a `project.*` event (contract `:1145-1147`, verified) re-resolves `not_linked` bindings and the event's own project, so Ship turns on in about 8 s. **t06:59-62** the field comment. **plan:797** the §10.1 row. Tests **t14:88** `not shared names the way forward`, **t07:441** `TestProjectEventReResolvesUnshared` |
| **M18** the rollback's environment join; pending rows forever | Part 1 was **already done**: rule 6's rollback candidate is joined *"by `same`"* (**t08:260-261**), and rule 1 gives Hub rows `environmentRow` through `SameEnvironment`. No change. Part 2: **t08:274-285** rule 7's age cap. A row is `pending` while `DeploymentsAt` is zero or no later than `confirmedAt + pendingGrace` (5 min, the skew allowance `pinSettle` uses, t07:277). After that it is the new kind **`not_found`**. The cap is measured against `DeploymentsAt`, never `now`, so a Panacea outage never calls a run gone. The row returns to `run` if a later read lists it (Panacea serves its live list when the durable read fails, `handlers/projects.go:1202-1207`, verified). **t08:85-92** the kind and `deploymentId`; **t08:101** `at`; **t13:111** the type; **t17:110**, **:115**, **:118** the heading, the **Not found** section, and Scans withheld. Tests **t08:344** `TestPendingRowAgesToNotFound`, **t17:245** `not found says so` |
| **M19** counts disagree with the code | **Drill routes: seven** (five reads, two streams; t12:56-101): **t12:13**, **:15**, **:138**. **`classify` callers: four**, verified in `desktop/src` (`EmptyDevelop.tsx:58`, `SessionPane.tsx:49`, `sessionVerbs.ts:86`, `useCreateSession.ts:38`). t13:21 and :308 already said four; **t13:354** said three and is fixed. **Pin cell: nine states.** The review counted eight; b09's *"pin not loaded"* row makes nine. **t15:25**, **:126-129**, **:198**. **`promote`: the review is wrong on the facts.** Panacea does set it: the history backfill copies `promote` and `rollback` from audit labels onto reconstructed `historical` rows (`clients/deployment_backfill.go:54-60`, `:170-178`, written through the store at `:273`, verified). What was false was plan 7P-4's *"the only `Trigger` values set anywhere"*. **plan:224-230** now says a live path sets four and the backfill sets two more. **t08:103-107** the trigger field keeps `promote` and adds `rollback`; **t20:495-498** the ask's *"no caller sets `rollback`"* is corrected. Test **t08:345** `TestBackfilledRollbackTriggerIsARun` (no rule reads `trigger`) |
| **M20** `ship.Config` lacks Briefs; `PinPreview` copies the Hub's rule | **t09:69-75** `Briefs` interface (`ForBranch`, contract `:1295`, verified); **t09:93-95** `Config.Briefs` (nil → `PinPreview nil`); **t09:157-166** `PinPreview` states the Hub's reserve rule (contract `:1015`, `:644`, verified) and that it is a preview; **t09:416-418** the step names `Config.Briefs` and its wiring. **t16:76-83** the Ship bar says *"should pin brief rev N (the Hub pins it at Ship)"*, never *"pins"*, and gives `no_complete_revision` its own words. Tests **t09:404** `TestPinPreviewMatchesTheHubsPin` (preview against the same Hub fake's `Reserve`, compared through `pin.briefKey`, because the Release carries the key, not the id: contract `:983-986`, verified), **t16:198** |
| **M21** a 429 or 3xx `KindUnavailable` matches no abandon branch | **Already closed** by b05 round 2 (R-2): the *"Any other error"* branch (**t09:306-313**) abandons every error no named branch takes, `KindUnavailable` with `0 < Status < 500` included. **t09:383** `TestOtherErrorAbandons` gains a `Status 302` case, the review's 3xx |
| **M22** a restart between reserve and retry reserves twice | **The review's first option is wrong on the facts, and its second only describes the defect.** A Hub key derived from the desktop's would replay the reservation, but the retry would still call Panacea again (a second build). And if the first process had already confirmed, the replay would leave the second build unpinned, with a `409 release_state`. The root cause is that the retry guard lives in memory. **t09:190-230** step 1 is now a **Ship journal** on disk: one file per `(ProjectID, IdempotencyKey)`, written with `writeAtomic` at `reserved`, `shipping` (before `Actor.Ship`), `started`, `done`. A retry continues from its stage and never reserves or builds twice. A `shipping` record abandons with step 6's no-answer sentence and never calls Panacea again. **t09:107-110** `Resume` restarts the confirm loop of every `started` record, which a restart used to lose (the same root cause, not a tangent). **t09:227-230** the one window left, and why it builds nothing. **t09:96-98** `Config.JournalDir`; **t09:16** files; **t09:328-329** step 7; **t09:420-422** `main.go` (`statepath.Dir("deploy")`, `statepath.go:30`, verified: it creates nothing). **t10:37-43** `withReservation` takes the key, so rollback and rebuild are journalled too. **plan:557** the file row. Tests **t09:378-382** (five), **t10:198** `TestRollbackIsJournalled` |

### Deviations, declared

- **M19, `promote`.** The review says Panacea sets no `promote` trigger. Its backfill does, so
  the enum keeps it. The fix corrects the plan's claim, which was the false statement, and adds
  `rollback`, which the same backfill writes.
- **M18, a new kind.** *"Say not found"* needs something on the wire. A `pending` row with a flag
  would call a row *pending* that is not. `desktop/src/deploy/types.ts` is task 13's new file,
  not `store/types.ts`, so UI contract §2 is untouched.
- **M22, the journal.** Covered above. It needs no Hub amendment. It also closes the confirm loop
  lost on restart, which the review did not name.
- **M17, the event re-resolve.** Without it, a Share would leave Ship off until the binding's
  60 s recheck, while the line under Ship still says *"not a shared project"*.

### Mutants

No code changed in this batch, so no mutant was run. Each new test names the mutant it kills.
The two I expect a re-review to find surviving:
1. **Resume abandoning `reserved` records.** `TestJournalRetryAfterRestartReservesOnce` retries
   without running `Resume` first, and no test runs `Resume` over a `reserved` record. So a
   `Resume` that abandons such a record, and turns the retry's continue into a refusal, passes
   every test.
2. **The age cap's boundary.** `TestPendingRowAgesToNotFound` uses 10:04 and 10:06 around a
   10:05 cap, so `>` and `>=` at exactly `confirmedAt + pendingGrace` both pass.

### Unverified

- The Hub's Go names: `hubapi.ForBranch`'s fields (`BranchBrief`, `ProjectBrief`, `Head`,
  `LatestCompleteRevision`), taken from the wire (contract `:770-771`, `:644`), and whether the
  Hub fake's `Reserve` applies `:1015` in full (archived briefs included).
- Whether Hub task 35's `CreateProject` emits `project.remote_added` as well as
  `project.created`. The re-resolve rule handles either.
- Whether `openShare` can be detected as absent at run time or only at compile time. t14 renders
  the button only where the store has it. If the store's type always has it, the fallback
  sentence is dead code until task 42 lands.
- `writeAtomic`'s durability (a rename; whether it fsyncs) bounds the journal's crash window.
  It was not read.

### Fix batch b10, round 2 — the re-review's open issues (2026-09-24)

Source: `.superpowers/sdd/p7/plan-rereview-b10-1.md` (two Important, five Minor). Line numbers are
at the commit that carries this section. Hub contract re-read at hub `53583f2`: `:53`, `:268` and
`:990` unchanged.

| Issue | Change (file:line) |
|---|---|
| **I-1** a retry abandons a live Ship | t09:218-230, **the live set**: step 1 claims the key before it reads the record and holds it until `done` or the confirm loop ends; a live key answers `409 conflict` with the act's own lock sentence, or the live loop's `202 confirmed:false`, and is never settled. `Resume` claims the same way (t09:107-110, :250-258). t09:103 the `Orchestrator` holds the live set; t09:383 the route's `409 conflict` row. t10:41-47 `withReservation` holds it for Rollback and Rebuild. Tests: t09:419 `TestJournalLiveRetryConflicts`, t09:420 `TestJournalLiveConfirmLoopNotDoubled`, t10:218 `TestRollbackLiveRetryConflicts`. Plan:557 names it in the file map |
| **I-2** the restart test contradicts the stage rules | t09:418 rewritten: the test reserves `rl-1` on the Hub fake and writes the `reserved` record through the journal's writer (the process stopped after the reserve, before `shipping`); a new `Orchestrator` with the binding **unresolved** runs `Resume` (record still `reserved`, zero `Abandon`), then `Ship` → one `Reserve`, one `Actor.Ship` from the record's fields, `Confirm(rl-1, {pipelineRun})`, record `done`. Kills M-3b |
| **M-1** `confirmedAt` not on the wire | t08:137-138 `ReleasePin.confirmedAt` (Hub contract `:990`); t13:108 in `desktop/src/deploy/types.ts`, so UI contract §2 is untouched; t17:245 the test asserts the sentence draws `confirmedAt`, never the row's `at` |
| **M-2** doc comment and mismatch code | t09:138-144 the comment settles by stage, names the four non-replay cases and the pre-reserve re-check, and says `New` is the only prune; t09:257-258 the same in step 1. t09:234-238: `400 bad_request` is this route's own code (the control plane has no `idempotency_mismatch`); the Hub's is `422` (`:53`, `:268`) |
| **M-3a** age-cap boundary | t08:346 exactly 10:05:00.000 → `pending` |
| **M-3b** `Resume` abandoning `reserved` | t09:418 (I-2's rewrite) |
| **M-3c** rule 6's join by `same` | t08:338 second case: a rollback reserved `prod` makes the `production` row's `current {source:"rollback"}` |
| **M-4** the record too thin to continue from | t09:207-215: `reserved` carries `reserve` and `act` (task 10's `ActSpec`); `started` carries `{run, answer}`. t09:242-246 a `shipping` abandon uses the `NoAnswer` words of `act.kind`. t10:31-62 `withReservation` takes an `ActSpec`, not closures; t10:113, :146 Rollback and Rebuild build one. Tests: t10:217 `TestRollbackShippingAbandonsWithRollbackWords`, t10:216 `TestRollbackIsJournalled` asserts the settled answer |
| **M-5** the `openShare` seam | t14:35-39 `DeployLens` takes `onShare?`, the seam; t14:25-28 `AppShell` passes it once Hub task 42 has merged; t14:72 and :94 the row and the test use the prop (absence injected, no cast); plan:797 the §10 row names the seam and who wires it |

#### Deviations, declared

- **M-2.** The mismatch stays `400 bad_request` rather than becoming `422`: adding a code to the
  control plane's set is a Go contract change for a case that is a client bug. The text no longer
  calls it the Hub's rule.
- **I-1, the live-key sentence.** A live retry answers with the act's own lock sentence, not a new
  one, because to the user it is the same act still running.
- **Durability.** `writeAtomic` does not fsync the directory (`registry/filestore.go:460-495`,
  read now). t09:265-269 declares it: a process stop keeps every record; a power cut can lose the
  newest stage. Not changed here, because it is the shared `writeAtomic` (ruling R2).

#### Mutants

No code changed, so no mutant was run. The two I expected to survive the round-2 tests, both now
given a killing case on paper:
1. **`Resume` ignoring the live set.** Nothing ran `Resume` while an act was live. Killed by
   t09:419's `Resume`-from-the-hook case (zero `Abandon`).
2. **A settled rollback that loses Panacea's answer.** `TestRollbackIsJournalled` checked the
   confirm and the call count, not the answer. Killed by t10:216's `warmth` and `image` assertion.

#### Unverified

- Whether `hubapi.ReserveRequest` and `deploy.ProjectRef` marshal as JSON unchanged (the record
  stores both). The Hub's Go names are not written yet.
- The live set is per process. Two control planes on one state directory would each hold their
  own; the control plane is single-instance on loopback, which this batch did not re-verify.
