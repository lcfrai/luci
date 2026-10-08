# Windows recovery and storage acceptance

This is an acceptance procedure, **not evidence of completed installed testing**. `codex/luci-production-runs` currently has the separate
LOCAL-006 checkpoint `2dcf245daca98d93e9cf7a40399b8fc78f4608a2`. Use the published recovery/storage correction revision and matching package for
LOCAL-005/CLOUD-009.
No remote operation should change Dylan's local machine.

| Priority / gate | Existing evidence to retain | Required native retest and pass evidence |
| --- | --- | --- |
| P0 / LOCAL-004 + LOCAL-006 state selection | Local tester reports all nine module tests passed at `c46405fc7092d93bbc5514442eacd4b175d14518`, with HEAD/all 1,807 tracked files unchanged. At `2dcf245d`, LOCAL-006 now has ten unedited native module passes and all four original junction cases reject InvalidData with target bytes preserved; HEAD/all 1,808 tracked hashes unchanged. No app/sidecar/Docker ran. | Carry forward the exact corrected module and original reproduction passes; on an affected later revision, verify ordinary file and nested path under a file refused; `sessions`/`instance` junctions refused both with and without an earlier regular identity marker. Repeat `invalid_explicit_state_fails_without_default_fallback` and `identity_marker_cannot_hide_later_linked_state_paths`. Preserve fixture target, source and normal-state hashes. Then separately build/run full native Tauri own-sidecar: `runtime_info` selects exact explicit state, first-owner setup, reopen, persistence and only owned-sidecar shutdown. Full Tauri lifecycle remains open. |
| P0 / CLOUD-009 recovery mechanics | API/management 242 race passes/three expected skips; product 29 race passes/no skips; each kept 644 Go/module inputs unchanged. Controlled tests and reopening are not actual installed controller death. | Native file locking and named-pipe backend tests below; then actual installed delayed Create and delayed immutable-ID Start with controller death, real daemon denial of the old request, lost fence acknowledgement, interrupted finalization, fresh consent and a subsequent useful task. Retain exact request chronology and proof identities privately. No capacity release based on a timeout or empty inventory. |
| P0 / never-started task retry | Actual HTTP reserve → abandon → reopen → fresh reserve and queue passes with no execution ledger/backend call. Stale abandonment after actual dispatch refuses. | Through the native UI, reserve a task without starting it, retain/cancel it, close/reopen the same isolated state and authorize fresh reserve/queue. Prior output remains inspectable; the new attempt has a different identity. No unexpected runtime start. Repeat with an actually pending launch: ordinary retry must refuse and preserve the recovery entry point. |
| P1 / LOCAL-005 cached storage refresh | 66 focused frontend tests, TypeScript and controlled Chromium at 1440/390 pass. Actual Windows confirmation/member denial/restart persistence passed on the older source app separately. | Native WebView Settings/portal: while cached Refresh is pending, Close, Back and setup choice work; no full-readiness POST occurs; a late reply cannot restore departed UI. Explicit readiness still works separately. Capture actual HTTP and exact process-start trace for Git/Docker/AWS/Azure; a one-second sampler cannot prove brief processes were absent. |
| P1 / shared storage and admission | Native read-only probe and private confirmation already reported passed; controlled cross-process readers make zero native fallback calls. | Current exact engine/Desktop/VHD/settings observations after supported relocation; current-user private ACL, same confirmation after restart, current owner only, member403 without private path. Several actual workspace processes share one gateway producer: instrument exact WSL probe-start counts plus cache ages/revisions, not sampled process absence. Confirm fresh separate guest and lowest-local-volume headroom; changed/stale/missing observation blocks new heavy work without killing existing agents. |
| P1 / sustained local usefulness | Linux installed four-project runs passed at earlier exact packages; source-browser checks, short native observer checks and synthetic soak remain distinct evidence. | Fresh current candidate: native 30-minute four-project workflow with normal browser/editor present, real context amendments, dependency-aware execution, review/integration, restart/failure recovery and pause/cancel/resume. Record actual concurrency, provider mode and costs, host available/commit, owned processes, Docker separately, UI latency and ordinary-app responsiveness. Keep live-provider reasoning/result quality separate from deterministic-provider mechanics. |

## Exact source selection and isolated paths

After the implementation thread publishes a full integrated revision, set its
40-character hash explicitly. Refuse a placeholder or dirty existing checkout;
never reset, clean or overwrite a tester's retained checkout. A fresh clone on E:
is an option when the previous checkout contains evidence:

```powershell
$candidateRevision = '<published-full-40-character-integrated-commit>'
if ($candidateRevision -notmatch '^[0-9a-f]{40}$') { throw 'A published exact candidate is required.' }
$sourceDir = Join-Path 'E:\Luci-local-acceptance\sources' $candidateRevision
if (Test-Path -LiteralPath $sourceDir) { throw 'Use the existing checkout only after checking its exact HEAD and clean status; do not overwrite it.' }
git clone --no-checkout https://github.com/lcfrai/luci.git $sourceDir
if ($LASTEXITCODE -ne 0) { throw 'Source retrieval failed.' }
git -C $sourceDir fetch origin codex/luci-production-runs
if ($LASTEXITCODE -ne 0) { throw 'Branch retrieval failed.' }
git -C $sourceDir checkout --detach $candidateRevision
if ($LASTEXITCODE -ne 0) { throw 'Pinned revision is unavailable.' }
if ((git -C $sourceDir rev-parse HEAD).Trim() -cne $candidateRevision) { throw 'Source revision mismatch.' }
if (git -C $sourceDir status --porcelain) { throw 'The candidate checkout is not clean.' }
```

Use existing Git, Go1.26.3, Node/npm lockfile dependencies and Rust/native Windows
build tools. Preserve the working SDK setup; an earlier registry-discovery concern
is not a reason to reinstall it. Record versions and before/after tracked hashes.
Keep caches, `TEMP`/`TMP`, fixture data and reports on the roomy acceptance drive
using **process-local** configuration. Preserve prior environment values without
printing them. Never copy production credentials or private state into fixtures.

Source/portable API isolation uses `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR` and an
unused loopback `LUCI_ADDR`. Native own-sidecar isolation uses the first two plus
`WEBVIEW2_USER_DATA_FOLDER`; leave `LUCI_API` unset so the native launcher starts
its own sidecar and chooses its own free loopback port. Read actual `runtime_info`
`apiUrl`/`stateDir`/`startupError`; do not assume `LUCI_ADDR` controls that mode.
Setting `LUCI_API` intentionally tests external-service mode, which must not stop
that external service. Follow the existing isolated procedure in
`docs/luci/LOCAL-WINDOWS-ACCEPTANCE.md`; preserve both normal roots
`%APPDATA%\ai.lcfr.luci` and `%LOCALAPPDATA%\Luci\state` before/after.
Do not bypass `scripts/test-installed-windows.ps1`'s disposable-runner guard.

## Native controlled checks: exact Go names

Run from the published checkout's `control-plane` directory, with evidence outside
the checkout. These commands do not enable the real-Docker opt-in. Native race
instrumentation needs its own supported Windows C toolchain; the plain native
commands below do not claim a Windows race run.

```powershell
$env:GOMAXPROCS = '2' # Dedicated acceptance shell; restore its prior value afterward.
go test -p 1 -count=1 -json ./internal/api ./internal/product -run '^(TestSwarmFencedHTTP|TestSwarmFencedRecovery|TestSwarmFencedRetirementHistory|TestSwarmPendingLaunch|TestProjectSwarmFence|TestProjectSwarmPendingLaunch|TestExecutionLifecycleLockExcludesLiveControllerAndReleasesAfterProcessDeath)'
if ($LASTEXITCODE -ne 0) { throw 'Native controlled recovery checks failed; retain the JSON output.' }
go test -p 1 -count=1 -json ./internal/backend/dockerbe -run '^TestLaunchFence(Acknowledges|Lost|Invisible|Rejects|Identity|Permanent|Proof|Late|Resource|Stopped|Newly|Renamed)'
if ($LASTEXITCODE -ne 0) { throw 'Native controlled backend checks failed.' }
```

The exact decisive test names include:

- `TestSwarmFencedRecoveryRefusesActualLiveAllocatorWithoutWaitingOrEffects`
- `TestExecutionLifecycleLockExcludesLiveControllerAndReleasesAfterProcessDeath`
- `TestSwarmFencedHTTPPrivateIntentSurvivesUnpublishedProductState`
- `TestSwarmFencedHTTPPrivateProofReopenVerifiesWithoutSecondFence` (waits beyond the five-second proof window)
- `TestSwarmFencedHTTPQueuedUncertainLaunchRetiresQueueAfterProof`
- `TestSwarmFencedHTTPUnclaimedAbandonReopenAllowsFreshConsent`
- `TestSwarmFencedHTTPStaleAbandonCannotMintUnclaimedRetirement`
- `TestProjectSwarmFenceRecoveryInterruptedProofAndFinalAck`
- `TestProjectSwarmPendingLaunchUnclaimedProofCannotCoexistWithClaimOrRecovery`

`TestLaunchFenceLiveDockerAcknowledgmentAndLateRequestBoundaries` is a separate
explicit opt-in using `LUCI_TEST_DOCKER_FENCE=1`. It creates only fresh nonce-owned
inert empty-root test objects, uses the actual selected local engine, and cleans
only those positively identified test objects without force. It performs no
image pull/build or agent start. It is useful native named-pipe/real-daemon proof,
**not installed controller-death or useful-task acceptance**. Schedule it only
in the agreed fixture window, preserve prior `DOCKER_*` configuration, verify the
actual local Linux-container engine, and restore/remove the opt-in afterward.
Do not point it at a guessed remote engine or relabel preexisting production
markers as test resources.

The actual installed fault driver in `scripts/acceptance/launch-recovery` is
currently **Linux-only** (`/proc`, Unix socket and actual SIGKILL/reaping). Do not
claim its wrapper is a Windows test. A reviewed native Windows fixture transport
and exact-process-death adapter is still needed to reproduce those installed
scenarios safely on Docker Desktop. It must delay real requests/drop replies,
not fabricate successful API/Docker responses. Required order:

1. Alive original allocator: explicit recovery promptly refuses without intent,
   marker, destruction or released capacity.
2. Actual death during delayed Create/Start, then reopen the same private state;
   explicit supported recovery records intent before effects. Delayed original
   Create receives real409, old immutable-ID Start receives real404.
3. Lose a successful fence Create reply: reservation remains held. Explicit retry
   creates a different acknowledged marker under the same intent.
4. Kill the exact owned controller after durable proof/cancellation but before
   final lease release; restart and explicitly continue. Reverify the same saved
   acknowledged marker, then exact lease/queue finalization, with no duplicate
   intent or task. Files/context and original acknowledgement remain unchanged.
5. A separate newly authorized task completes and is reviewed/integrated through
   the application. Recovery itself is never counted as task success.

Production markers are permanent. No prune, normal Stop, registry deletion,
ledger edit or protocol-0 relabelling is an accepted recovery or test cleanup.
The 64 permanent intent/fence slots and 4,096 retirement records are bounded;
legacy/exhausted cases have no supported automatic reclamation path in this
implementation. Preserve output and report the limitation rather than implying
an operator can safely reset it.

## Storage and performance follow-through

Exact existing read-only native probe: `TestWindowsLocalStorageReadOnlyProbe` in
`./internal/resourcebudget`, configured only with the actual operator-selected
`LUCI_WINDOWS_STORAGE_PROBE_VHD` and a fresh private
`LUCI_WINDOWS_STORAGE_PROBE_REPORT`. See `docs/luci/WINDOWS-DOCKER-STORAGE.md` for
the scoped command. The controlled sharing test is
`TestSharedWindowsStorageIsolatedProcesses` in `./internal/backend/dockerbe`;
it proves file/cache semantics with exact probe counters, not production WSL
performance. Measure real multi-workspace probe starts separately.

Use the tracked `sample-windows-owned.ps1` with a private manifest of exact root
PIDs plus creation identities, current owned instance/image and verified disk
roles. Record explicit idle/load/restart/settle markers; retain host/process and
container metrics as separate scopes. The eight-sample native PowerShell check
already reported does not establish Luci memory/latency or Docker observation.
A stale resource snapshot or too little host RAM must block new heavy work;
relocating a VHD does not supply additional RAM or justify changing admission
limits. No cloud billing or public deployment is prerequisite to these local gates.
