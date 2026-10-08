# Continuing product-quality work

The continuing objective remains [GOAL.md](GOAL.md), judged against
[PRODUCT-QUALITY.md](PRODUCT-QUALITY.md). This is an implementation worklist,
not a replacement completion standard. The owner resumed the full objective on
8 October with explicit agent-swarm requirements. Last confirmed persistent
platform status is **blocked**; this session exposes no goal read/edit/activation
control. An active working thread does not establish platform reactivation or
completion. Independent implementation continues; no individual batch closes
the full objective.

The owner has authorized an ongoing Windows test/correction loop in
[LOCAL-TEST-LOOP.md](LOCAL-TEST-LOOP.md). The local thread tests pinned isolated
revisions and reports stable issue IDs; the implementation thread acknowledges,
fixes and sends exact revisions for retest while other testing continues.

## Current acceptance checkpoint — 8 October 2026

Verified storage/cleanup source checkpoint:
`8f78804ff318ab744226f139df5998cbd6e0d0ff` on
`codex/luci-production-runs`. This adds Windows backing-storage confirmation,
shared observations and safe same-sweep task cleanup. The local tester reports
173 native Go outcomes, seven Git-fixture outcomes and 53 frontend tests passed,
with no failures or skips, plus an actual read-only Windows storage probe.
These are focused native checks, not installed performance or provider quality.
Earlier native lifecycle and fixture corrections at `2819ce9e` remain verified.
It is not the finished local candidate.
The earlier `e81b5860` checkpoint established the following scoped foundation. Source checks, a clean portable package,
portable startup/persistence checks and the corrected ten-minute controlled
scheduler run pass. Installed Home bulk actions, partial permission failure and
exact-run navigation also pass against that package.

The installed Linux Docker/Codex four-project run now passes against the clean
`2819ce9e` package: four native one-shot tasks exit naturally, three automatic
capacity handoffs precede review, all four outputs receive browser review and
integration, and a service restart preserves the same live worker. The separate
cancelled queued task allocates nothing. This is a deterministic provider with
actual native CLI/container execution, not live-model reasoning. The 428.478-second
workflow and 445-sample resource report are retrievable under
[evidence](evidence/2026-10-08-2819ce9e-installed-swarm.md). Previous failures remain
preserved. The clean matching `8f78804f` Linux package has 1,048 extracted files
verified against its source and 347 licence components with none missing. Its installed four-project workflow passed in 276.837 seconds versus 428.478
seconds in the retained baseline, with all four output hashes equal and no
compiler overlap in the newer run. This is one scripted-provider comparison,
not an isolated cleanup speedup or live-model quality claim. CLOUD-009 implementation continues separately from
that immutable package and the local tester's pinned checkout.

| ID | Priority / owner | Observed defect and expected behavior | State / evidence |
| --- | --- | --- | --- |
| CLOUD-001 | High / cloud implementation | Workspace preparation holds the source lock, making queue reads and controls return immediate HTTP 409 `branch_busy`. Private queue reads and pause/resume/cancel/priority must remain usable while source-dependent operations stay guarded. | Fixed at `9a84e740ad85e4a03540f2dfb082f98cb60604c5`. Linux production-router race/fresh-permission checks pass. Local tester reports five focused native Windows Go tests passed, zero failed/skipped (Go1.26.3, real Git): held-lock availability/CAS/protection, fresh revocation, primary-branch admission, delayed observations after pause/cancel and independent item controls. No child was allocated by the held-lock test. Ready for installed slow-allocation/UI retest; full local acceptance remains open. |
| CLOUD-002 | High / cloud implementation | A completed swarm task uses an interactive CLI that can remain open and retain capacity. Task execution must end or visibly request intervention, release owned resources when safe, and let other projects progress without manually stopping every completed task. | Source correction verified: supported native one-shot commands, explicit saved autonomy, Ask/Plan refusal before allocation, retained context, and Awaiting review status. Direct interactive terminals remain available. Final frontend 2,375 tests/typecheck/build and affected backend/reaper/race checks pass. Matching Linux installed natural-exit and queue advancement pass at `2819ce9e`; Windows installed acceptance remains open. |
| CLOUD-003 | Medium / cloud implementation | Transient source locking removes a previously loaded project summary. Retain explicitly labelled last-recorded context only for `branch_busy`, hide stale next actions, and clear private summaries on access denial. | Source correction, 23 focused Home tests and TypeScript pass. Matching installed `9898cf5d` Home witness passes real bulk actions and controlled HTTP 409/403/500 handling; actual held-source-lock browser retest remains separate and pending. |
| CLOUD-004 | High / cloud implementation | Stop can race an unfinished native backend-create request. An exited registry row and momentary backend absence must not free its reservation or permit source integration while the original allocation can still appear. | Source correction verified: durable unresolved/settled launch phases, serialized owned launch/Stop, and positive ownership/settlement checks for release, admission reconciliation, replacement and source review. Full API suite passes; final 143 API/harness race outcomes pass with unchanged inputs. Ambiguous backend calls retain reservations and are not claimed automatically recoverable. Matching installed/native retests remain open. |
| CLOUD-005 | P1 / cloud implementation | Linux VM disk space does not establish Windows backing-volume headroom. Desktop settings and generic VHD attachment flags do not reliably discover the active image. | Source implemented: explicit host-operator confirmation pinned to engine/Desktop/Windows/file/settings identities; separate minimum across mounted writable local volumes and guest capacity; private shared observations produced by the gateway, consumed without repeated native probes by workers. Controlled storage race checks: 19 passed; Windows test executables cross-compile. Frontend: 2,394 tests passed, TypeScript and desktop/phone controlled browser checks passed. At exact `8f78804f`, the local tester reports native private ACL/cache and controlled multi-reader checks passing, plus the actual read-only storage probe (four volumes; C: limiting at 3,722,629,120 bytes). The later actual Windows source-app host-owner confirmation, member GET/POST403 without private path leakage and confirmation revision1/hash persistence through restart passed. LOCAL-005 tracks the separate cached-refresh UI defect. Actual WSL process-start counts and installed readiness/admission remain open. |
| CLOUD-006 | High / cloud implementation | Installed `9898cf5d` cannot prepare an explicitly saved Bypass swarm because `GET /v1/harnesses` omits the advertised autonomy capabilities. The UI correctly refuses unsupported or absent capabilities. | HTTP projection corrected for autonomous-task support, allowed permission modes and notices, including existing custom-model/Docker Ask metadata. Real HTTP catalog regression and full API suite pass: 663 test outcomes, five opt-in skips, 39.474 s, with all 573 Go/module inputs unchanged. Receipt: `artifacts/cloud006-full-api-verification.json`. Native public HTTP projection passed at `2819ce9e`; matching Linux installed dispatch passes at `2819ce9e`; Windows installed dispatch remains separate. The failed installed attempt allocated no agent and remains retained as failed evidence. |
| CLOUD-007 | P1 / cloud implementation | VFS creates an init and writable layer, so reserving only one image copy understates pending container disk demand. | Source correction: two copies for VFS, conservative one-image allowance for supported overlay drivers, explicit refusal for unknown drivers/overflow, and pending reservations across restart. Affected race checks: 70 outcomes passed, zero skipped/failed, 6.898 s. Matching Linux VFS installed admission passes at `2819ce9e`; Windows admission remains separate. |
| CLOUD-008 | P1 / cloud implementation | Reaper status updates from stale observations can overwrite a concurrently restarted session; normal task exit also waits an extra sweep before container cleanup. | Source and focused native correction verified at `8f78804f`: compare the exact observed registry generation before writing exit, require one positively settled task launch, and remove only the immutable owned stopped container without force. Retain the task lease until confirmed backend absence so transient removal failures remain retryable and stopped-container disk stays reserved. Admission also retires completed manual-task leases after an exact absence check; manual tasks have no queue release callback. Focused race checks pass. The complete combined Go race suite passes 2,835 outcomes (27 explicit test skips), with unchanged 1,786 repository file hashes and HEAD/status; the full Windows API cross-compiles against those bytes. The local tester reports the affected native same-sweep cleanup, immutable-ID removal, restart and failed-removal cases passing. Matching installed four-project timing passed at `8f78804f`: 276.837 seconds total, same four output hashes. Native-exit observation to cleanup was 9.106–26.682 seconds; baseline exit-frame timestamps are unavailable. Retain the 428.478-second baseline and full breakdown; one run does not isolate a causal speedup. |
| CLOUD-009 | P1 / cloud implementation | A controller crash during backend-dispatching can leave an unsettled lease indefinitely. Stop can remove the child without releasing capacity, Forget refuses, and late created-only containers lack a positive settlement receipt. | Confirmed by source review; implementing explicit reviewed retirement for newly protocol-marked launches, with a per-child cross-process lifecycle lock, original engine identity, durable intent and an acknowledged permanent same-name Docker fence. Existing ambiguous legacy launches remain unsupported; they cannot be relabelled as safe. Controller death, timeouts and empty inventory are not proof. Preserve the lease, isolated checkout and evidence until fencing succeeds. Existing reaper retry guidance overpromises for this case and must be corrected. This blocks a full local completion claim; focused CLOUD-005/008 testing can proceed. |
| LOCAL-001 | P1 / cloud implementation | Native lifecycle allocation returns unexpected 500/worktree-unavailable and unknown task state. `swarmPath` uses Windows backslashes against a portable-slash guard. | Source corrected: portable slash construction, reservation/integration path-error handling, and an existing-parent/two-checkout regression. Fresh exact `9898cf5d` native logging confirmed the cause. Affected Linux API race checks pass; verified for the original native reproductions at `2819ce9e`; see the reported native receipt. Original batch: 11/15 top-level tests passed, four failed; 27 subtests passed, zero skipped. See LOCAL-002 for original batch HEAD drift, and LOCAL-003 for the separate timeout. |
| LOCAL-002 | P1 / cloud implementation | Lifecycle fixture ignores `swarmPath` error; empty worker path lets file writes and Git inherit the package directory, mutating the test checkout. | Source corrected: checked path results and explicitly registered temporary-directory ownership before any write/Git, with a Git discovery boundary. Negative subprocess cases run inside a disposable parent repository and preserve its bytes. Affected Linux race checks pass (67 outcomes, one intentional subprocess-entry skip); full product suite also passes (739 outcomes, two skips), with all 578 Go/module inputs unchanged. Verified at `2819ce9e`: native rejection checks preserve the disposable parent repository and the entire test checkout. The prior local isolated checkout advanced from `9898cf5d` to `8f4deb8a4d7300a875b4e18ff44cf5e222fca9b1` at 08:04:12Z, adding only `control-plane/internal/product/alpha.txt`; original C: checkout untouched. Preserve contaminated checkout and receipts. Later short-TEMP reproduction used source-identical Go blobs at that changed HEAD, not pristine `9898cf5d`. |
| LOCAL-003 | P1 / cloud validation | Held-Create regression returns 500/context deadline instead of 201. Fixture ReadyTimeout 50 ms derives only 100 ms for Create while the barrier spans real Windows route/Git work. | Source corrected: explicit 10-second Create scenario budget matching barrier waits, preserving production deadlines. Affected API race checks pass, including unchanged deadline/cancellation contracts; verified for the original native reproductions at `2819ce9e`; see the reported native receipt. |
| LOCAL-004 | P1 / cloud implementation | Native own-sidecar startup ignored explicit state. First correction `656a1ae1` honors the override but Windows retest found an ordinary file accepted as a directory. No native app was launched against user data. | **Reopened / source corrected:** native minimal Rust module reported8passed/1failed at `656a1ae1`; all other override/default/junction cases passed. Every existing marker parent now explicitly must be a directory, including a file above a missing nested override. Updated Linux module checks pass9/0 with unchanged captured inputs; native retest and full own-sidecar compile/lifecycle remain open. Use the subsequent exact correction handoff and isolated WebView instructions. |
| LOCAL-005 | P2 / cloud UI | Cached Refresh Docker storage status calls the parent full readiness check (Git/Docker/AWS/Azure) and locks Close/Back/setup navigation. Source call chain and busy UI were confirmed at `8f78804f`; process-start trace was unavailable. | Fixing: isolate cached status refresh from explicit readiness checks and keep navigation responsive; preserve fresh ownership, late-response guards and authoritative readiness. Native host-owner confirmation, member GET/POST403 with no path leakage, and revision1/hash persistence across restart passed separately. |
| LOCAL-006 | P1 / cloud implementation | An early identity marker returned before validating later sessions/instance paths. Native Windows diagnostics at `656a1ae1` accepted two junction-backed roots after adding identity, while rejecting both without it. No app or downstream write occurred. | Source corrected: validate every recognized marker path before accepting current or legacy state. Four-case regression reproduced both unexpected acceptances on Linux before the fix; all ten corrected Rust module tests pass with unchanged source hashes. Native junction/file rejection and full own-sidecar lifecycle remain open. Receipt: [marker validation](evidence/2026-10-08-local006-marker-validation.json). This correction also retains the LOCAL-004 non-directory parent check published at `c46405fc`. |
| LOCAL-ENV-001 | P1 / local environment | Docker's live VHD was on nearly full C:, despite spacious source/cache drives; guest free space alone did not prove backing-volume headroom. | **Verified by local tester:** supported UI move completed to E:, active VHD/UI/CustomWslDistroDir agree, old C: VHD absent, same engine ID/version/context healthy. All 107 container IDs/states, 196 image rows (178 distinct IDs), 155 volume IDs and four running IDs are unchanged. C: 74.20 GiB, E: 903.27 GiB, guest 886 GiB free; cold verified backup retained. A fresh native read-only probe at `8f78804f` reports four volumes, C: limiting at about 74.2 GiB. Only 4.38 GiB host RAM is currently available, so actual confirmation proceeds without runtime/model allocation. Product confirmation and full workload gates remain separate. |

The local tester also reports nine scoped Windows source-browser checks passed
at `2819ce9e`: isolated startup/owner creation, four separately saved project
purposes, switching/search, scoped draft recovery, and restart persistence.
Home axe reported zero violations (31 passing rules); an 800 px view had no
horizontal overflow. All 1,694 tracked-file hashes and HEAD remained unchanged,
and owned services/browser processes were stopped. The local setup auto-install
route was deliberately avoided; there were no downloads or worker requests.
See the [reported browser receipt](evidence/2026-10-08-2819ce9e-native-browser-reported.json).
This is source-browser and manually authored context evidence, separate from
installed Tauri, live agents, sustained resources and the full local milestone.

Before the initial Windows acceptance handoff: finish the task lifecycle,
integrate and publish the exact corrected revision, pass the installed
four-project workflow with owned-resource observations, and include sanitized
reports and reproduction helpers in retrievable source. Native Windows,
live-provider quality and the sustained realistic workstation workload remain
local acceptance gates. No cloud-only merchant or deployment prerequisite blocks
these steps. See [LOCAL-TEST-LOOP.md](LOCAL-TEST-LOOP.md) for coordination.

## Swarm coordination and context integration — 8 October 2026

The interface must be a real interactive application using polished shadcn/ui
styling and consistent accessible controls. Add scoped cross-project and bulk
actions where useful, with explicit effects and partial failures. The originating
local thread `01a1199f-7500-7843-b02b-081081e536fa` owns first Windows acceptance;
notify it only after an integrated retrievable candidate and concrete four-project
handoff are ready. See [GOAL.md](GOAL.md) for the authorized notification contract.

The newest local requirement is sustained progress across **four or more
projects** while the owner can comfortably use their **31.2 GiB RAM Windows
computer**. Add a cross-project overview, persistent isolated context, global
resource-aware fair scheduling with priorities and provider limits, bounded idle
and long-running memory use, and a repeatable four-project mixed-workload test.
Measure all owned workers/containers/dev servers/tests/browser processes plus
host pressure; UI-only memory does not establish responsiveness. Explicit local
budgets must come from measurements. Preserve the same model for later cloud
offload. This requirement is part of local acceptance, not a deferred cloud gate.

**Immediate delivery: local first, with Windows handoff.** Complete local-relevant
work without waiting for merchant setup, hosted infrastructure or public DNS.
Native Windows and live-provider outcomes remain explicit local acceptance
gates; prepare exact reproducible steps when this environment cannot execute
them. The later cloud phase must use Luci itself on its source, with the owner's
supplied access and app-visible progress/integration review.

Research GSD-Core, related GSD solutions and Nimbalyst against the current
architecture, using primary sources and explicit licensing. Implement the
strongest applicable improvements in the existing coherent project journey.
Acceptance must exercise planning/dependencies, parallel delegation, durable
context, isolated work and integration, intervention/review, and interruption
recovery. Record measured time, resource efficiency and verified result quality,
distinguishing controlled-agent evidence from live-model quality. Research and
a roadmap alone do not complete this work or the full product goal.

### Verified foundation checkpoint

The first integrated swarm foundation now supports reviewed dependency plans,
task-specific immutable context, independent task repositories, bounded change
and retained-output inspection, explicit integration, and interrupted-attempt
reconciliation. Controlled child processes verify concurrent execution and
restart binding; they do not establish live-model quality. The safe Windows
development launcher has controlled-process checks and Windows compilation;
native execution remains for the local tester.

The frozen-source full checks passed: **2,288 frontend tests across 135 files**,
TypeScript, and **2,387 Go test/subtest outcomes with 23 explicit skips**. Source
hash snapshots match all 502 Go and 366 frontend inputs. Detailed results and
preserved initial failures are in
`artifacts/quality-swarm/full-baseline/verification.json`. The controlled tiny-job
benchmark showed concurrency but no speedup; see the comparison document.

This historical foundation checkpoint was not the local acceptance candidate.
The following integration adds global fair resource admission and durable
queues, the cross-project overview and scoped action controls, the shadcn/ui
migration, and bounded terminal/session memory. Installed and native acceptance
remain separate from their source verification.

### Four-project integration pass — source verified, installed acceptance open

The next working-tree batch implements a durable task queue with atomic reviewed
batch consent, dependency checks, fair project turns, per-item revisions and
explicit recovery. Manual and queued launches share durable capacity leases;
current identity and project permissions are resolved again before execution.
Resource observations separate physical/cgroup host capacity from the Docker
execution host. Shared host policy and Windows Docker storage observations have
focused integration and cross-compilation checks. Existing explicit account/provider limits
remain in force; a four-agent workspace ceiling does not promise four live
agents.

The overview now has bounded project summaries, attention and scoped bulk queue
actions. Per-workspace project-stage preferences restore the selected project
without retaining prompts or credentials in navigation storage. Canonical
shadcn controls replace the old button/dialog implementation while retaining
upstream MIT provenance and existing compatibility behavior. Background queue
controls, consent changes, uncertain responses and retirement recovery have
focused regressions. The final integrated Go suite passes 2,600 test/subtest
outcomes with 25 explicit skips; all 32 tested packages pass. All 2,361 frontend
cases across 143 files, TypeScript and the web build pass. Source inputs are
unchanged across both runs. Earlier failures remain retained: a source-location
assertion was corrected to check its contract, and catalogue navigation now
imports its module before the test measures response fencing. Test timeout
thresholds were not widened. Export/provenance regressions pass all 23 cases.
Matching installed acceptance remains open; source results are not a Windows handoff.

Terminal panes mount only the visible/focused subset and keep at most eight
idle attachments. The actual-source Chromium 50-session switching test reduced
peak retained connections from 32 to 11, settled to 3 after the grace period,
and released all after unmount. A separate overload witness verifies explicit
failure/manual retry with a bounded buffer. These are connection/buffer results,
not measurements of full application RAM. The corrected real-WebSocket relay
preserves overload closure before cancelling its peer connection. The retained
Docker candidate includes these sandbox changes; its exact exported bytes and
actual installed runtime still require verification before handoff.

Working evidence for this pass is under ignored `artifacts/quality-swarm/` (queue UI and
contract checks, terminal-limits, navigation-regressions and source-export checks)
and `artifacts/resource-probes-test.log`. The sustained four-project controlled
process test is exercising actual scheduler/allocation paths. Its first
ten-minute run failed the restart witness because an expected child had exited;
all 601 samples and the failure are retained. Diagnostics reproduced a fixture
startup deadline of 100 ms; the corrected fixture uses the production readiness
budget. Its synthetic execution capacity now admits the intended three 4 GiB
requests while retaining 25% headroom. The corrected short run passes with six
successful results and one deliberate exit-2 failure. The corrected ten-minute
workload also passes: 602 samples, three live workers in all 600 workload samples,
six successful results and one deliberate failure. See
[SWARM-CAPACITY-ACCEPTANCE.md](SWARM-CAPACITY-ACCEPTANCE.md) for measured scope.
Earlier failures are not passing endurance evidence. Controlled telemetry/provider
fixtures do not establish native Windows capacity or live-model output quality.
The broader goal and all unverified local acceptance gates remain open.

An additional recovery correction covers a stop after durable capacity binding
but before a child is registered. It requires current edit/control authority,
the exact retained operation, fresh complete registry/backend observations and
a provable pre-backend launch phase. A durable retirement receipt precedes
capacity release, and interrupted finalization can be retried explicitly.
In-flight or ambiguous backend creation remains retained for inspection. This
change passes affected product/API/management race checks, including a real
allocator/recovery overlap and restart between retirement and finalization.
Windows publication uses write-through replacement and cross-compiles. The
shared pending-recovery state blocks new manual/queued attempts until durable
finalization. The UI reads back a failed response once, clears stale consent,
and exposes explicit completion without replaying the mutation.

## Current findings and required outcomes

| Finding | User consequence | Required correction and evidence | Status |
| --- | --- | --- | --- |
| Run-status wording does not explain what is observed or what reconciliation does. | Recorded checks can appear to conflict with “No observation”, and the status action is difficult to distinguish from reloading history. | Explain the existing action and its effects, align current recovery wording, and retain execution/permission boundaries. | Check run status and its history/evidence distinction are implemented with wording-only frontend/backend changes. Independent review, affected frontend cases, TypeScript and74 focused Go outcomes pass. Matching installed769d5bf8 passes all4 overview/amendment/composer groups at1440/390; visual review confirms the wording, and independent runtime/cleanup verifies all1477 package entries unchanged. The status action itself was not invoked in this walkthrough. |
| Reviewing the current plan scrolls the entire application out of view. | The stage/header disappears and blank space replaces part of the phone work area. | Keep the cockpit within the viewport while preserving its intended planning, keyboard, short-height and active-session scrolling. | Exact769 trace proves document displacement; first c3 correction still scrolls the hidden shell72px and fails the durable header guard. Corrected ordinary clipping with explicit auto-scroll overrides passes235 source cases and all6 installed deeb55ad groups. Four real desktop/phone handoffs retain root/shell0 and headers; native workspace, labelled exited-session phone layout and genuine400% short-height scrolling/keyboard return pass with visual focus review and independent cleanup/all1477 entries unchanged. The firstdeeb native-probe loading/selection driver failure remains retained; no thresholds were weakened. Session-layout proof is display-only, not live coding. |
| Opening the generic change discussion clears an occupied answer’s question binding. | The text survives, but its browser draft and later Send no longer identify the question it was answering. | Preserve the occupied message and reply target when opening discussion; keep explicit Clear reply and empty generic discussion behavior. | Source-confirmed through the actual Change the plan → Discuss a change with Luci path. Both before-fix answer/send endings fail. The minimal correction preserves exact browser state through the actual CTA, recovery and attributed Send; independent review and clean full2,243 frontend/TypeScript checks pass. Matching installed dca39101 confirms exact occupied text/reply/browserrecord and focus through Discuss at1440/390, followed by Keep/Replace. |
| A stale run header still recommends release evidence while the overview asks for plan review. | Two prominent surfaces disagree about the next action after an amendment. | Put current review status first and qualify old lifecycle/reason as recorded evidence, preserving active work and accepted history. | Independent first-exposure screenshot reading found this at installed7eac68f4; source confirmed. Current-review header, qualified historical evidence and on-demand selected-task title comparison are corrected. Routine retained-read refresh preserves the open comparison; confirmed denial retires it. Reply-target and build-location copy are clarified. Independent reviews and clean full2,243 frontend/TypeScript checks pass. Matching installed dca39101 verifies current/historical guidance, real lazy plan read and complete title comparison at1440/390. Follow-up screenshot reading resolves the original ambiguities; native scroll ergonomics remain a separate diagnostic. |
| Project overview can recommend historical checks or drop the verified build identity after an amendment. | The overview disagrees with run detail about current scope, and its preview action can lead to generic deployment without the selected run. | Reuse canonical context freshness while preserving historical evidence; make the next action open the exact run for review and its existing checked-build handoff. | Corrected with canonical freshness, bounded summaries and exact run review. Independent review additionally caught a cached-run handoff; explicit inspection now refreshes that exact run before its checked deployment action. Original regressions retained; combined310 frontend cases/TypeScript and1,080 API/product race outcomes/four skips pass. Installed7eac68f4 passes all3 overview/amendment/exact-handoff groups at desktop/phone using labelled historical check setup. No model/check command/deployment occurred. |
| Selecting a suggested answer can replace an unsent amendment. | A detailed message and its browser recovery copy can be lost when the person answers a different question. | Protect the occupied composer until an explicit choice, preserve question binding and avoid automatic sending. | Corrected with explicit Keep/Replace, preserved durable text/reply binding, retired stale choices and guarded keyboard focus when an answered question removes the opener. Original shipped failure and separately caught uncommitted focus failures retained. Independent review and combined310 affected frontend checks/TypeScript pass. Actual installed7eac68f4 Keep passes, but Replace fails to return editor focus despite updating the text/question. Phone composer assertions remain unexecuted. The actual dialog close lifecycle correction is independently reviewed and included in clean full2,243 frontend/TypeScript checks. Matching installed dca39101 verifies both Keep and Replace focus with exact text/reply/storage at1440/390. The original failed evidence is retained. |
| Runtime configuration offers private-variable changes without the required project grants. | A reader can type changes the server will reject, and confirmed read loss can leave old metadata visible. | Mirror actual read/edit/secret authority, preserve private in-memory work through ordinary access refresh, hide denied content, and fence late results. | Corrected with project/account/workspace ownership, current read/edit/secrets guidance, private in-memory draft continuity and retired late responses. All58 affected checks and TypeScript pass; independent source review finds no remaining scoped blocker. Combined frontend validation passes2,192 cases across131 files plus TypeScript with434 unchanged inputs. Installed54d4da87 passes all6 real-grant/delayed-original-response groups at desktop/phone. Private drafts remain in memory, the intended save persists, newer names survive stale reads, and independent cleanup plus visual review pass. Focus-event scaffolding is labelled; no deployment/model claim. |
| Git delivery reads and status refreshes can overwrite drafts or hide an already successful operation. | A typed remote/name can be lost; successful connect/create/push followed by failed readback appears as failure only. Insufficient project grants are not reflected in controls. | Preserve drafts and newer state, separate confirmed operation success from status-read failure, expose exact permission remedies, and retain explicit publication/credential controls. | Corrected draft/read ownership, navigation protection, operation success reporting and project/workspace permission guidance. All63 affected checks pass. Independent review caught and closed a pending-request lock defect in the uncommitted correction, separately retained from the original shipped defects. Included in the combined2,192 frontend pass. Installed54d4da87 passes all7 real-role/local-origin/response-ordering groups, with independent live process and complete package/cleanup proof. Visual inspection catches a misleading generic service-down message after successful attach and failed readback; the narrow factual read-failure copy correction passes26 cases and all7 installed7eac68f4 groups, with visual and independent runtime/cleanup proof. |
| Plan full-page screenshots include extra blank space beyond the actual viewport. | A screenshot can imply an unusable scroll region that the person does not encounter. | Compare actual viewport and full-page captures with native wheel behavior, ancestor geometry and reachable task controls before changing layout. | Separate exact603 diagnostic classifies this as capture inflation for the tested desktop/phone flow: html.scrollHeight exceeds844/1080, but the hidden outer overflow keeps window.scrollY0 and the footer fixed. Native planning scroll reaches Task2, New task and Save; inert outer-wheel probes expose no blank area. No product change. |
| Team navigation offers creation that the current member cannot perform and calls its worker “This PC”. | The sidebar contradicts the main workspace and misstates where work runs. | Mirror existing create permission and identify only the exact default team host while preserving actual identity, state and custom names. | Corrected;32 focused checks and independent source/accessibility review pass. Included in the combined2,149-case frontend pass. Matching installed `603bd900` passes all7 managed-entry/sidebar groups at desktop/phone, with clean rendered permission/host details and independent owned cleanup. |
| A saved-document reload can overwrite a draft restored while the read is pending. | Define, Design or the implementation plan can silently lose recovered context; a later edit can replace the retained browser copy too. | Make reload ownership and pending state explicit, prevent restore/edit/save against an unsettled read, and preserve recovery on failure. | Confirmed source sequence after failed Save → Reload → early Restore. Define/Design corrections pass91 focused cases and the combined2,149 frontend cases with TypeScript. The implementation-plan follow-up passes51 relevant cases, TypeScript and independent review, including unfinished task titles and fresh-revision merging. Installed `603bd900` passes all three real stale-save409 → unchanged-held-GET → deliberate merge/review → current-revision saves, including the unfinished task title. Independent cleanup passes; the separately recorded Plan capture observation is a screenshot artifact for the tested flow. |
| An older connection refresh can reverse the displayed provider choice after a confirmed change. | The person sees the wrong model billing account selected for their next session. | Make confirmed mutations and all catalogue reads share one response-ordering guard; retain current errors and visible refresh progress. | Reproduced after explicit Gateway off. Corrected selection/error/event ordering passes focused coverage and the combined2,122 frontend cases plus TypeScript. Installed `adbc7467` passes four actual UI/delayed-original-response groups at1440/390, with zero provider/session activity and independently verified cleanup. |
| An existing account cannot reach invitation acceptance on a managed installation. | The service hides Manage teams until membership exists, so a valid invitation cannot unlock the account. | Expose scoped invitation acceptance and account recovery at the managed entry screen; keep private content hidden until the bound membership is confirmed. | Confirmed by provider/control routing and the authenticated invitation endpoint. The correction passes74 focused/related checks; the combined frontend passes2,122 cases and TypeScript. Exact team receipt plus refreshed membership is required before entry, with account/binding fences and sign-out recovery. Installed `adbc7467` passes six functional groups, but post-join screenshots expose stale403 errors from premature hidden-child reads. The admitted-once correction passes76 affected checks; matching `2d1f4c8c` passes all six installed groups with zero stale403 text, clean post-join rendering and usable390 px entry. Owned runner cleanup passes; independent confirmation is recorded separately. |
| Older deployment-history responses can overwrite newer ready releases. | Overlapping refreshes or polls can remove the selected release and close a healthy preview after it opened. | Fence superseded results/errors and prevent background polls from starving slow reads, retaining closure on the latest confirmed unavailability. | Source-confirmed ordering gap in DeployLens refresh, polling and operation-final refresh. The original failing regression is retained. Correction passes85 affected cases, including slow polling, current removal and stale errors; combined frontend2,122 and TypeScript pass. Installed `adbc7467` passes original-response acceptance: old200/500 results cannot close the newer ready preview or lose its unsaved input; latest unavailability still closes it, and the latest error is shown. The saved cloud-history display fixture makes no deployment claim. |
| Captured agent inputs cannot be fully inspected by a teammate after an amendment. | A delivery receipt and old revision numbers do not show the complete agreed definition, design and plan; viewers cannot use the privileged terminal as a substitute. | Add deliberate read-only inspection of verified captured documents, retaining original scope, team read authorization, useful loading/error/legacy states and keyboard recovery. | Installed `9c585ec3` passes12 actual captured-document inspections, including a true read-only viewer and phone inspection while canonical B remains visible. Original bundle/history/package and exact captured A remain unchanged; independent cleanup passes. Source/API/revocation evidence retains its original scope. |
| Active reviewed agents can reread newer canonical context after a teammate amendment. | The displayed old build scope can disagree with the instructions actually read by its running agent. | Capture reviewed canonical bytes into a separate read-only session bundle, bind delivery, verify reuse, explain older sessions and prove same-child rereads remain unchanged while collaborative edits stay visible. | Confirmed by actual API A→B regression: six delivered files change on the same child before the fix. Correction passes full Go2,165/22skips, affected race1,097/4skips, Windows/Darwin cross-compilation and UI2,075 cases across full/corrected-fixture runs plus TypeScript. Installed old85 reproduces drift; corrected98 independently proves same-child A remains after actual UI B, with one Create/Start and verified cleanup. Overall98 attempt still fails phone terminal-area gate; original failed dimensions were not retained. Subsequent9c human inspection and603 native lifecycle now pass their separately recorded scopes; live model quality remains open. |
| Captured-input guidance leaves insufficient terminal area at 390 px. | The phone can show the delivery notice but fail to leave a useful coding surface. | Measure the retained supported layout, identify the constraint and preserve readable context plus a reachable terminal without weakening the original minimum. | Installed `9c585ec3` passes at390 px with a374 ×416 px terminal body and overview open. Same-selected Open reveals and focuses the retained terminal; two delayed real-response cases preserve newer task focus and Define navigation. Original98 failure and later labelled geometry/overlay diagnostics remain retained. |
| Native resume returns an internal error after a real coding session and service restart. | A person cannot continue the existing conversation even though the original child and captured inputs survived restart. | Preserve the stopped provider history and captured context in an independent resumed session; diagnose the failure using actual native state and verify successful reuse and cleanup. | Genuine native state first reproduced transient launcher-link rejection; its correction resolved HTTP500. The pinned0.160 split/unified comparison then confirmed provider settings were dropped at fork. Corrected `603bd900` now passes all8 installed lifecycle groups: actual conversation/context continuation, restart, independent Forget and corrupt/stale refusal before allocation or destruction. Exactly2Create/2Start and2 real context reads; complete941-file package and owned cleanup are independently verified. Original failed receipts remain retained. |
| Subscription denial points users to the operator even when the owner can recover payment access. | The displayed service state and recovery advice disagree, leaving the person without the right next step. | Distinguish operator suspension from unverified or elapsed subscription access, give owner/member-specific remedies, retain cleanup and avoid implicit payment requests. | Installed `9c585ec3` passes all3 actual UI cases: owner and member subscription remedies plus operator-suspension precedence. No provider traffic, commands or Docker allocation; independent fixture cleanup passes. Seeded billing observations are explicit, and both prior harness failures remain retained. Payment/access policy is unchanged. |
| Native 400% browser zoom collapses the Develop workspace to zero height. | Keyboard focus can reach work that is entirely invisible below fixed shell controls. | Keep the short-height journey in reachable scroll flow, retain bounded active terminal/Canvas surfaces, and verify native 200%/400% zoom plus ordinary phone/desktop layouts. | Installed `4c6b895f` passes native 200% and restores content reachability at 400%. The remaining full-width disclosure clipping is corrected by an inset outline; matching clean `e7e4dc41` passes all29 native400% Tab stops with full physical screenshots, unchanged actual zoom geometry and reachable content. Original failures retained. |
| Restoring a stale structured draft replaces a teammate's independently changed fields. | Recovering one person's constraints can silently undo another person's agreed outcome. | Preserve the actual original baseline; combine independent definition/design/task edits; require explicit choices for overlapping edits or unknown legacy baselines. Verify two people through the actual UI, retaining old run scope while reviewing the new scope. | Verified in the matching `a10d941e` two-person browser journey, including readable combined content before explicit restoration. Overlap, legacy and reference-repair regressions also pass. |
| Prototype review hides the screen's expected behavior and can reuse a consumed team browser invitation after navigation. | The person cannot compare the working result with the agreed experience or reliably return to it. | Keep routes, states and outcomes alongside the actual prototype; record source-bound human observations as unverified draft context; obtain a fresh explicit browser connection. | Verified end to end in `7ff66d3b` attempt 5: actual behavior, planned width, fresh invitation, source-bound observation and exact-screen amendment saved as an unapproved draft. |
| Deployment operations can say completed when a successful HTTP response carries a failed operation status. | The person receives a false success and no useful recovery path. | Display the actual operation outcome and remedy. Keep returning users connected to their retained release, source and health, with explicit supported cleanup/replacement controls. | Operation, permission and ownership checks pass. Matching `a10d941e` browser acceptance verifies retained release discovery, explicit owned stop, reviewed replacement and preview continuity through access refresh. |
| An unsent discussion disappears on remount; an uncertain send can lose its request identity. | Context is lost or the same intent can become a second paid request after refresh. | Scoped private draft recovery and durable exact-request identity; no automatic resend. Resolve acceptance or authoritative non-dispatch without racing a delayed original request. | Verified in the combined `7ff66d3b` attempt 5: unsent draft reload and pre-admission network-fault recovery retain exact text/request with zero model calls. Backend concurrency/receipt checks cover accepted and delayed requests separately. |
| Collaboration evidence stops before a complete amendment during active work. | Mechanism checks alone do not show people can understand and continue the combined workflow. | Two people preserve concurrent contributions, see old active scope versus new agreed context, renew review, build the amended behavior, retain a meaningful regression check, and inspect/accept the matching preview. | Matching `a10d941e` browser journey verifies active-scope understanding, both contributors’ edits, renewed review, actual CLI/checks, matching preview and acceptance. Provider remains a deterministic fixture, not live model reasoning. |
| Repository analysis can return a consequential question without a proposal, but the UI hides the question. | An imported application's discovery stops without explaining the decision needed to continue. | Show the question and editable answer, then explicitly continue analysis using its authoritative prior result and the selected connection. Preserve source/review checks and recover the reply draft. | Implemented with durable exact-request recovery, canonical answer ancestry, provider-revision consent and actual workspace-actor ownership. Complete source checks and six installed `e04fa35c` groups pass; the separate visual refinements below continue. Live model reasoning remains separate. |
| Stripe returns to the app without reopening subscription status. | A customer cannot see whether payment was verified or how to continue after checkout or cancellation. | Return to the authorized team's Service and data view, with explicit verification guidance and no entitlement inferred from the URL. Require an explicit choice before changing workspace. | Implemented with sign-in continuity, membership checks, explicit workspace choice and draft protection. Persisted return-URL version preserves existing Stripe idempotency parameters. Eight installed `e04fa35c` return groups pass using labelled billing fixtures; merchant acceptance remains separate. |
| Data controls invite operations outside the person's project permissions. | People encounter avoidable permission errors without knowing which access is required. | Explain read/edit/deploy/secret requirements, preserve drafts during access refresh, and fence operations after actual revocation. | Implemented across data, database, functions, email and domain controls. Delayed responses and downloads are fenced after permission changes; successful database recovery links directly to application deployments. Seven installed `e04fa35c` grant/refresh/revocation groups pass with labelled database display fixtures. |
| Completed analysis questions and restored answers sit below long settings/evidence, with no demonstrated transition from an option to editing. | The person can miss the consequential decision or fail to find the next action although the underlying request succeeds. | Prioritize the question, answer and explicit next action; focus the answer on an explicit user action without stealing focus on asynchronous completion. | Verified in the matching `7f39ea9e` installed journey: the reloaded question precedes the answer form, explicit answer actions focus the fully visible editor at 1440 and 390 px, and technical evidence stays behind disclosure. Asynchronous completion preserves the current scroll/focus. |
| The default expanded project overview consumes nearly half the phone viewport. | Context crowds out the actual work and the answer/send flow requires unnecessary nested scrolling. | A compact default at narrow widths, with an obvious context disclosure that respects explicit choices and retains drafts. | Verified at 390 × 844 in `7f39ea9e`: compact overview is 61 px and usable analysis area is 633 px, up from about 289 px. Explicit open/close retains the same draft DOM and text without sending work. Responsive/focus regressions also pass. |
| Optional hosting discovery delays local Platform readiness and completed local operations. | A slow external project list hides already-ready Docker/Data controls or leaves a stopped local service looking busy. | Admit verified local project/Git/capabilities independently, retain explicit provider loading/error/retry and stale-response guards, then prove actual local action completion during genuine provider delay. | Both original deferred-provider regressions fail before;34 component tests and TypeScript pass after. Final full frontend2,072/128files and TypeScript pass with unchanged inputs. Matching `85f830e0` passes all3 installed groups: actual local view/stop while providerpending,503/explicitretry, unchanged identity/context/permissions/source, clean owned teardown. Two original harness failures remain retained. |
| Platform permission refresh presents unconfirmed access as missing grants. | An authorized person is briefly told to ask an owner for permissions they already have. | Distinguish loading, transport failure and a confirmed missing grant without changing authorization or discarding work. | Verified in seven installed `7f39ea9e` permission groups: pending access says it is being checked; failed delivery gives retry guidance; actual secret revocation does not falsely claim deployment editing was revoked. |
| A programmatically selected team-settings tab is clipped on a phone. | Billing returns to the right panel but hides part of its selected label and focus ring. | Reveal the selected/focused tab within its own horizontal strip, including on resize, without moving the dialog or document. | Verified in eight installed `7f39ea9e` billing-return groups. At 390 px the selected tab and full focus ring fit within the strip; document/dialog scroll is unchanged. Four scoped scans have zero violations. One phone contrast item remains automated incomplete; a retained-image manual assessment finds unobscured text at 12.47:1, matching two saved passing observations. This is a sampled assessment, not full accessibility conformance. |
| Deployment and hosting fields use bare native controls against inline labels. | Core operation forms look inconsistent and provide poor spacing for reading and touch interaction. | Use locally imported platform styles, readable label spacing and responsive Luci fields while preserving validation and actions. | Verified in five installed `7f39ea9e` maintenance groups. Repair fields measure 350 × 44 px on a phone, with separated labels and complete keyboard focus outlines. The actual failed deployment row and its controls are visible; seven scoped accessibility scans have zero violations or incomplete items. |
| Data table, read-only and file fields still use bare native controls. | The data workflow has inconsistent readability, sizing and disabled states despite correct permission handling. | Apply scoped Luci field tokens and spacing, preserve native checkboxes and responsive columns, and verify real keyboard focus and draft retention. | Verified in clean `4c8012c1`: seven permission groups and four desktop/phone field observations pass. Both controls are 44 px high, labels and keyboard focus rings are fully visible, drafts survive, and 18 px checkboxes and zero horizontal overflow are retained. |
| Opening a new portable package silently reuses an older running package. | An update appears successful while the person is still using old code and UI. | Compare the running service with the selected portable package through its existing local ownership boundary; explain how to close the previous launcher before upgrading, preserving state and processes. | Five installed `1b0d8043` upgrade checks pass: legacy-service refusal preserves the process and 26 private/project files; explicit replacement preserves account/session/context; same-package reuse and cleanup pass. Separate verification binds the retained old extraction to all 939 archive members. Native Windows acceptance remains separate. |
| Removing a requirement leaves an invisible invalid link in its design screen. | Saving the design is blocked with no visible link to remove; deleting the whole screen would lose unrelated work. | Show unavailable linked requirements with explicit removal; retain all other screen content and existing approval semantics. | Source correction passes the 47-test Design batch and complete 1,887-test frontend suite. Five installed `1b0d8043` groups pass: actual definition removal, explicit unlink, retained screen fields/history, save/reload and separate approval. |
| Typing a comma in responsive widths immediately removes it. | Normal typing joins two widths into an invalid number although filling the entire value at once works. | Preserve raw edits and give recoverable validation before saving parsed widths; keep dirty-state and draft recovery intact. | Per-keystroke, exact draft reload and teammate width-conflict regressions pass. The same five installed groups verify ordinary keystrokes, the unfinished comma, blur/tab retention, reload/Restore, numeric persistence and renewed approval. |
| A generic untouched starter can become an approved design handoff. | A role-based application's design can advance to Develop without showing how its users actually complete the proposed tasks. | Keep starter material distinguishable from an authored, reviewable design; guide meaningful screen/role decisions before approval without inventing an architecture or accepting a rubber-stamp checkbox. | Explicit authoring, saved people/outcome guidance, unsent help and separate draft/approval guidance pass the full 1,890-test frontend suite and four installed `bfcd5cb1` groups. Four desktop/phone accessibility scans have no violations or incomplete findings; no live-model design claim. |
| The compiled service ignores help/version/unknown arguments and starts normally. | An operator asking for help can initialize default state and attempt to start a service. | Handle informational and rejected arguments before process protection, configuration or service setup; preserve normal no-argument startup and exit codes. | Early argument handling passes 21 focused outcomes and eight isolated source-binary checks. All eight exact packaged `bfcd5cb1` checks also pass: invalid config is bypassed for informational/rejected arguments, no fresh state is created, secret-shaped values are not echoed, and ordinary no-argument refusal is retained. |
| Manual questions route to chat and disappear from the overview once answered. | People lose the direct path to an existing answer field and cannot see recorded answers across stages. | Target the question's existing Define editor and retain saved Q&A with draft/reviewed status in the shared overview. | Implemented; complete 1,944-test frontend suite and type checking pass. Four installed bb720e51 continuity groups pass; explicit UI answers/reviews and recovery use the archive-bound app. |
| Custom check commands and unfinished arguments disappear on navigation or reload. | Consulting the agreed design loses the verification work being prepared. | Recover scoped setup drafts without restoring execution consent or applying old criteria to changed context. | Implemented; complete 1,944-test frontend suite and type checking pass. Four installed bb720e51 continuity groups pass; explicit UI answers/reviews and recovery use the archive-bound app. |
| A prepared run still invites coding after its reviewed context changes. | The visible next step contradicts the amendment; clicking it reaches an avoidable backend rejection. | Project current context freshness separately from historical run state, preserve active work and offer a useful review action. | Read-only projection and guidance pass full source suites, including 481 product-package race outcomes; stale history and active sessions remain intact. Four installed bb720e51 run-guidance groups also pass with real context amendment and preserved history. |
| A new material question blocks execution while its build still invites coding. | The project overview and build disagree about the decision needed before proceeding. | Project the private decision blocker separately and open its existing scoped question without replacing drafts or sending anything. | Read-only projection, exact-question navigation and both overlapping-read orderings pass HTTP/UI coverage in the current full source suites. No implicit send or composer replacement; four installed bb720e51 groups verify projection and exact focus over explicitly seeded private question history, preserving unsent and recovered replies with zero execution. |
| A local verification draft blocks asking Luci for help. | The command being prepared has no shared Save operation, yet the composer demands saving or discarding it. | Distinguish local command setup from unsaved shared context; preserve navigation/source guards and explain that commands must be explicitly included in the message. | The `6e7797f3` correction passes the complete 1,979-test / 126-file frontend suite. Four installed groups pass there and again on `f388ad44`, verifying the explicit local-draft boundary and editable unsent help; the missing provider/model independently disables Send. |
| Empty coding panes squeeze verification recovery into about 185 px on a phone. | Reading and comparing exact commands requires repeated scrolling through fragments, although the actions work. | Give no-session planning and checks one continuous scroll area; retain active session layouts. | Installed `6e7797f3` recovery uses one scroller and 525 px on a phone versus 185 px before; `f388ad44` retains that result. Visible heading/action scans have no violations or incomplete findings. |
| Develop says there is no saved design after a structured design handoff is approved. | The next stage contradicts the review the person just completed. | Describe the actual structured handoff and its review dependencies; distinguish the optional sketch. | Independently reproduced on clean `bb720e51`; the `6e7797f3` correction passes full source checks and installed proof. Canonical handoff facts and exact plan focus pass again in `f388ad44`. |
| Review implementation tasks leaves the plan collapsed. | The primary next action reaches the stage but does not expose the work it names. | Open and focus the scoped task destination after guarded navigation, preserving drafts and read-only review. | Independently reproduced on clean `bb720e51`; the `6e7797f3` correction passes full source checks and installed proof. Canonical handoff facts and exact plan focus pass again in `f388ad44`. |
| A phone overview stays expanded after opening the exact task plan or answer. | The named destination receives focus but has too little room for the actual work. | Fold the overview to its stage bar only after an explicit exact destination succeeds; preserve cancellation and background behavior. | Verified in clean `f388ad44` first-attempt 4/4: phone plan and exact Answer destinations retain focus while the overview collapses; the plan uses 525 px with no driver scroll/focus. Desktop stays open. The narrow panel correction passes 86 tests and type checking, separate from the preceding full suite. |
| A viewer can type a new implementation task that cannot be saved. | Read-only review creates an unsaveable browser draft and unnecessary navigation guard. | Gate the new-task field and submission consistently while preserving an existing editor draft after downgrade. | The source checks and six-group installed `c6e28ec8` journey pass. Viewer input remains disabled without creating a draft or navigation guard; an existing editor draft survives pending access and downgrade, then saves explicitly after permission returns. |
| A delayed project backup can download after access changes or its view closes. | Source and Git history can arrive after the interface has lost the permission or scope that admitted the request. | Fence delayed downloads and notices by current account/workspace/project and required permission; a later regrant must not revive an old result. | Implemented; 59 focused backup/history/delivery tests and the complete current source batch pass. Ordinary retained-grant refresh is preserved; unconfirmed completion offers an explicit retry. Six installed `c6e28ec8` groups pass. All three delayed backup/readback scenarios produce zero stale downloads; stable authority downloads one ZIP with verified source, Git and context history. Restore lifetime remains component-tested because its current dialog prevents navigation while restoring. |
| Git can commit files changed after the displayed review. | Another contributor's unseen changes become part of the commit offered for publication. | Bind commits to the reviewed branch, source and index; reject stale review before staging, preserve the message and retain edits made after the captured snapshot. | Implemented; exact captured source, branch/index conflicts, protected files, normalization metadata, split indexes, native recovery logs and read-only behavior have source regressions. The complete source batch and four installed `448275fc` two-person groups pass: a real teammate save causes stale refusal without changing HEAD/index, explicit refresh retains the message and commits exact reviewed bytes, and later amendments remain uncommitted. Internal capture races remain separately covered by source tests. |
| Native keyboard navigation clips the task field’s focus ring at the planning scroll boundary. | The active control is harder to locate even though it accepts input. | Keep the entire focus outline visible through native scrolling at desktop and phone widths, with and without active sessions. | Reproduced in installed `448275fc` permission attempt 2 and a bounded Chromium diagnostic at 1440 and 390 px. Scoped scroll padding is verified in the installed `c6e28ec8` journey at both widths, with full keyboard focus outlines and no horizontal overflow. A separate four-case native-scrolling diagnostic covers both scroll layouts. The original failure is retained. |
| General phase focus styling overrides the tab component’s inset indicator. | Keyboard focus loses its edge at a horizontally scrollable tab boundary. | Retain the existing tab component’s inset focus style while preserving ordinary button focus; verify the actual focused tab at desktop, phone and short-height reflow sizes. | Verified in `84095f9d` and the matching `4c6b895f` ordinary viewport run: complete internal-tab outlines, unobstructed hit targets and selected-indicator alignment. Native200% also passes; retained failures identify the original source of clipping. |
| A sticky Define/Design action footer covers tabs in a short-height viewport. | At reduced effective viewport height, requirements status and actions obscure the navigation needed to continue. | Keep the footer in document flow at short viewport heights, retaining ordinary desktop behavior and all drafts. | Verified on `84095f9d` and `4c6b895f`: footer no longer overlaps navigation and actual focused hit targets are unobstructed. The original equivalent-viewport failure remains retained; separate native200% acceptance also passes. |
| Three of 40 warm mobile phase transitions exceed the 100 ms acknowledgement target. | A few ordinary actions can feel briefly unresponsive even though navigation completes. | Profile the actual click-processing work, resolve a demonstrated cause and remeasure on a quiet matching package; retain outliers and measurement limits. | Shared geometry removes demonstrated duplicate measurements;71 focused and2,059 full frontend tests pass. Installed `4c6b895f`80-action input-inclusive ACK p95 is62.4/72.2ms and ready p95 is492.5/228.3ms desktop/phone. Two ACK and two ready misses remain. Trace analysis identifies request dependencies but no specific slow function; original stalls, instrumentation differences and host limits stay explicit. Gate remains open. |
| An owned Docker child created during an interrupted allocation remains Starting after service restart. | The build appears active indefinitely even though the agent never started, leaving no timely explanation of what happened. | Reconcile only the exact owned never-started child after the existing grace, preserve live/in-flight creation and run identity, and show bounded explicit recovery without starting another child automatically. | Verified in all four installed `4c6b895f` recovery groups: actual Create once, zero Start/provider calls, same identity/source through restart and actual grace, clear failed/blocked retry, exact child removal and explicit cancellation. Independent cleanup confirms zero owned resources and unchanged pins. Full affected race1,283/7skips and bounded independent source review also pass; original aborted fixture remains retained. |

The current corrective batch passes 2,051 frontend tests across 128 files,
2,120 Go outcomes (22 skips), 513 product-package race outcomes (one skip),
and type checking. Exact source inputs remain unchanged; original fixture/audit
and temporary-disk failures are retained beside the final passing logs in
`artifacts/quality-collaboration-boundaries/`. Its portable (six), launcher
(seven) and Git-review (four) installed groups pass. The permission journey is
verified in six groups on corrected `c6e28ec8`, which also passes six portable
groups and seven launcher outcomes. Its only application change from the full
source baseline is scoped scroll padding; type checking and exact rendered
regression checks pass. The four earlier failed driver/product receipts remain
recorded with their distinct diagnoses. The earlier compact-handoff receipt is
`artifacts/package-eval/0.5.0-f388ad44/context-continuity-attempt-1/result.json`,
with screenshots and `rendered-review.json` beside it. Six scoped accessibility
scans have zero violations; one clipped, nonfocused Define Answer label remains
an automated color-contrast incomplete. Initial account/context are labelled
API scaffolding; claimed navigation, saves/reviews and recovery are UI actions.
No model, command or Docker execution occurs. The complete 1,979-test / 126-file
suite belongs to `6e7797f3`; `f388ad44` adds the 86-test panel/type-check result
and matching installed proof. These results do not close real-model quality,
representative authentication/roles/durable data, managed-service or native and
public-release gates in [PRODUCT-QUALITY.md](PRODUCT-QUALITY.md).

Two further concrete findings have source corrections, with installed acceptance continuing:

- A failed repository import reserved its final project name. Correcting Git access and retrying the same name returned `project_exists`, leaving the entry flow stuck. Owned staging, private receipts and no-overwrite publication now pass four installed `7ff66d3b` browser groups: actual failed HTTPS clone, corrected same-name import with both original commits/files, lost completion response, and explicit read-only recovery without another clone. No private-host authentication or live repository-analysis quality is inferred.
- Structured draft comparison previously exposed only JSON. Definition, design and task recovery now show readable content and explicit choices, demonstrated in the `a10d941e` browser journey; exact technical records remain available through deliberate disclosure. Sketch recovery remains a separate opaque-document path.

The `2a60c1c0` source batch passes 1,701 frontend tests across 117 files, type
checking and production build, 1,997 Go test/subtest outcomes (22 skips), and
1,170 race outcomes across the three changed integration packages (five skips).
These checks do not replace the pending rendered journeys or live model proof.

Current implementation owners coordinate shared files before editing. Each
correction needs its scoped regression and rendered behavior where applicable;
failures stay recorded. Earlier `340f1590` results retain their original identities.

The clarification, billing-return and data-permission source batch passes all
1,874 frontend tests across 125 files, 2,045 Go outcomes (22 skips), and 1,230
changed-package race outcomes (five skips). Windows AMD64 and Darwin ARM64 API
test executables compile. The initial frontend run's obsolete recovery-copy
assertion and its focused navigation correction remain recorded alongside the
clean complete rerun in `artifacts/quality-clarification`. Installed acceptance
of the new batch remains separate from these source results.

## Gates that still need real external access

Live model reasoning and implementation, merchant commissioning, separately
billed model/infrastructure policy, owned managed/cloud hosts and domains, SMTP
delivery, signed distribution, and installed Windows diagnosis remain open.
GitHub currently rejects Actions runner admission for account payment/spending
reasons. No permission, credential, live result or production readiness is
inferred from elapsed time or from deterministic local fixtures.

After this worklist is verified, continue with the next unproven complete journey
and any independently actionable defect it exposes. Do not mark the overall goal
complete because these rows or a test suite pass.

## Current installed continuation evidence

The clean `2a60c1c0` and corrected `a10d941e` Linux packages each passed six
portable acceptance groups and seven launcher outcomes. Both have verified
archive/source integrity and licence evidence for all 346 components.
`a10d941e` was built from a clean isolated checkout while import work continued
separately. Its overall team-journey attempt is **not passed**: seven groups
succeeded, then the prototype handoff tab did not activate. Preserve this failed
receipt at `artifacts/package-eval/0.5.0-a10d941e/team-amendment-attempt-3/result.json`.

Earlier attempts remain recorded: attempt 1 stopped on an ambiguous driver
selector; attempt 2’s expected text was corrupted by missing UTF-8 metadata in
the fixture application. Both harness defects were corrected without weakening
outcome assertions. Separately, the deployment preview closed during routine
permission refresh; that product defect is fixed, with confirmed denial and
late-grant refusal preserved, and its actual browser regression now passes.

The rendered review also exposed an accepted-run summary that still repeated
its old acceptance instruction. The projection and guidance are corrected, retaining current invalidity and
health warnings; focused UI and actual HTTP run/health regressions pass. The handoff-tab interaction
is being reproduced separately before attributing or changing product behavior.

The clean `7ff66d3b` package passes six portable acceptance groups, seven launcher
outcomes and the four import/recovery groups above. Its archive and every installed
web file are independently bound to the recorded package provenance. Preserve
import attempts 1–3: Git identity setup, a wrong entry-button selector and a
child-exit callback race were driver defects. Attempt 4 passes strict cleanup
with exactly three import requests, three real clone starts and two recovery
reads. Four bounded accessibility scans have no violations; their contrast items
remain automated incomplete results. A separate entry-state manual diagnostic
supports the sampled header/help text contrast at both viewport sizes, with
original findings and scope limits retained. The full `7ff66d3b` team attempt 4
again passes seven groups and stops at prototype-to-handoff activation. Its trace
records no handoff pointer event. A bounded complete-layout reproduction identified
Chromium 151 sending the click into the preview iframe using its old pre-scroll
position. A fresh comparison with explicit scroll and settled tab/iframe geometry
delivered one ordinary click correctly. The corrected driver then passed all ten
combined groups in attempt 5; product handlers and behavior assertions were unchanged.
Both failed attempts and the direct comparison remain retained. This completes
the scoped fixture journey, not live-model or full production acceptance.

The `7ff66d3b` installed standalone PostgreSQL journey passes five groups,
including actual backup bytes, trusted reimport, restoration into a candidate,
rollback preserving both generations, and service restart. SQL fixture witnesses
verify the records independently of success text; they are labelled setup and
observation, not an application-authentication test. All fixture-owned runtime
resources were verified removed. Receipt:
`artifacts/package-eval/0.5.0-7ff66d3b/database-recovery-attempt-3/result.json`.
This baseline predates the new data-permission and recovery-navigation UI.
Full Supabase authentication, row isolation and stored-file recovery need an
adequate host; their prepared capacity/scenario record is
`artifacts/plans/data-operation-host-prerequisites.json`.
