# Local Windows handoff and acceptance

**Integrated candidate pending; the local milestone is still open.** The native
Windows corrections below passed at their named revisions. They do not establish
that newer code, an installer, four-project sustained execution or a live model
meets the release standard. The implementation thread must supply the exact
published commit and matching package before the next integrated batch; do not
substitute a moving branch tip or invent a future revision.

The originating local testing thread, `01a1199f-7500-7843-b02b-081081e536fa`, owns
execution on Dylan's machine. The cloud thread supplies source, helpers, fixes
and evidence; it does not start processes on that machine. Keep the wider goal
open: a polished local app, later connected to cloud, for building context and
turning coordinated agent work into reviewed, verified software. Cloud billing,
public DNS and hosted infrastructure do not block this local milestone.

## Carry forward verified native scopes

These are relayed local reports; the cloud thread has not independently read the
raw Windows artifacts. Preserve their original revision and scope even when the
same unchanged code is included in a newer candidate.

| Revision and evidence | Verified scope | Not established |
| --- | --- | --- |
| [`919cffe014d1104d9e65fa19351683d182355f36`](evidence/2026-10-08-native-919cffe0-reported.json) | Native release build; LOCAL-004 exact isolated state, ordinary-state hashes/file IDs/creation and last-write metadata preserved using refreshed metadata and read-only native handles; LOCAL-005 six real UI/HTTP refresh scenarios, navigation during held responses, no late reopening, exact confirmation/readiness actions. All 1,911 tracked hashes unchanged; owned processes/listeners zero, 107 original containers and four running workloads preserved. | Runtime installation, worker/model execution, sustained responsiveness or exact process-start tracing. The focused recovery batch passed 39 top-level cases and 58 subtests, including real Windows lock release after process death, but controlled backend fixtures are not installed controller-crash acceptance. One timed-out invocation remains in the history. |
| [`675ed402385b4082afda539b63fb0ff03565f58e`](evidence/2026-10-08-native-navigation-675ed402-reported.json) | Native build; LOCAL-007 warm/cold reopen, stable project-label selection with four distinct saved purposes, explicit disclosure, roving keyboard navigation, Windows Ctrl+K, and Keep editing. Four ordinary closes stopped owned app/sidecar/WebViews; final cleanup zero, normal state preserved, 1,921 tracked hashes unchanged and original Docker workloads preserved. | Human-speed click stress, latency, four complete app definitions, worker execution or crash recovery. Fixed-coordinate WebDriver actions included DOM reads. |
| [Earlier native packaging and lifecycle](evidence/2026-10-08-native-package-2dcf245d-reported.json), `2dcf245d` | Clean native build, matching sidecar/resources and unsigned NSIS; 616 native licence components, none missing. Module/junction corrections and initial isolated lifecycle were separately verified. | The installer was not executed. The historical 527.7 μs directory timestamp discrepancy remains unexplained; the later corrected-state passes do not rewrite that original result. |

The `675ed402` report preserves an evidence-helper filename collision and its
focused repeat with unique receipt names. It closes the corrected navigation
scope, not the broader release. Recheck affected behavior when its code changes;
do not label a carry-forward result as a fresh run at the next commit.

Actual installed Linux delayed-Create and delayed-Start/lost-ack recovery passed
at clean `919cffe0`, including real controller deaths, late Docker denials,
explicit recovery, a successor task and reviewed integration. See the
[installed recovery receipt](evidence/2026-10-08-cloud009-installed-recovery.md).
Native Windows crash scenarios remain separate. Earlier installed four-project
runs used scripted provider responses and one concurrent worker; they establish
neither model quality nor sustained Windows capacity.

## Prerequisites and preserve the existing machine

Use Windows x64, PowerShell 7 (the source launcher also supports Windows
PowerShell 5.1), Git, Go **1.26.3**, Node **24.19.0**, a current Edge/Chrome and
Docker Desktop with its **WSL 2 Linux engine** running. Native packaging also
needs Python **3.11+**, Rust with `x86_64-pc-windows-msvc`, Visual Studio C++ build
tools/Windows SDK and Edge WebView2; see the
[Tauri prerequisites](https://v2.tauri.app/start/prerequisites/).

The tester's existing Rust/cargo 1.99, VS Build Tools 2019/VC 14.29, Windows
SDK 10.0.19041, WebView2 and Python setup has built the documented native package.
Preserve that working toolchain; an old SDK registry-discovery concern did not
justify reinstalling or changing global registration. Use task-local E: caches
and `TEMP`/`TMP`, restore prior process environment values, and do not print them.
Do not reset/clean the user's checkout or delete state to remedy a failure.

**Storage relocation is already reported verified.** Docker Desktop 4.48.0
(207573), engine 28.5.1 and the same engine identity remained healthy after the
supported move to E:. Desktop UI, active VHD and `CustomWslDistroDir` agreed;
the old C: VHD was absent and the verified cold backup remained. The tester
preserved 107 containers, 196 image rows/178 distinct image IDs, 155 volumes and
four running workloads. Do not move the disk again, unregister WSL, prune images
or stop those workloads as part of this procedure.

Post-move C: 74.20 GiB, E: 903.27 GiB and guest 886 GiB were dated observations, not
current capacity. Later source-app available host RAM was about 2.91 GiB; an
earlier build-only sample reached 2.578 GiB. Neither is sufficient evidence for a
new worker budget. Measure fresh physical availability, commitment, guest disk
and physical backing-volume space before heavy work. Keep ordinary browser/editor
work open; a successful relocation or package build does not certify headroom.

Retain a private before/after Docker inventory using list-only output, for example
`docker.exe ps -a --no-trunc --format '{{.ID}} {{.Image}} {{.State}}'`, plus image
and volume IDs. Compare the same pre-existing identities and running workloads
afterward. Restrict test cleanup to positively identified resources created by
the acceptance run. Do not use global cleanup or full inspect/environment dumps.

No hosted Luci account, Luci subscription or cloud account is required. Use your
own provider connection; model API/subscription charges remain separate.

## Retrieve the exact candidate and start safely

Use [LOCAL-TEST-LOOP.md](LOCAL-TEST-LOOP.md) for the current fix/handoff message.
The source branch is `codex/luci-production-runs`
([PR #6](https://github.com/lcfrai/luci/pull/6)); the next batch still needs its
published full commit. Use a fresh E: checkout and keep older evidence:

```powershell
$candidate = '<published-full-40-character-integrated-commit>'
if ($candidate -notmatch '^[0-9a-f]{40}$') { throw 'Select the exact published handoff commit.' }
$checkout = Join-Path 'E:\Luci-local-acceptance\checkouts' $candidate
if (Test-Path -LiteralPath $checkout) { throw 'Preserve this checkout; select a fresh location.' }
git clone --no-checkout https://github.com/lcfrai/luci.git $checkout
if ($LASTEXITCODE -ne 0) { throw 'Clone failed' }
git -C $checkout fetch origin codex/luci-production-runs
if ($LASTEXITCODE -ne 0) { throw 'Fetch failed' }
git -C $checkout checkout --detach $candidate
if ($LASTEXITCODE -ne 0) { throw 'Pinned revision unavailable' }
Set-Location $checkout
if ((git rev-parse HEAD).Trim() -cne $candidate) { throw 'Revision mismatch' }
if (@(git status --porcelain).Count) { throw 'Use a clean candidate checkout.' }
go version
node --version
npm.cmd --version
docker.exe version
docker.exe info --format '{{.OSType}}'
```

The engine must report `linux`. Record full HEAD, tracked-file hashes and clean
status before/after each batch. A matching release must include its source commit,
archive hash, `SHA256SUMS`, `release-manifest.json` and dirty-source flag. Preserve
the complete package/resources; do not copy only a same-named executable.

The clone carries these instructions, sanitized [native919](evidence/2026-10-08-native-919cffe0-reported.json)
and [native675](evidence/2026-10-08-native-navigation-675ed402-reported.json)
receipts, the [resource helper/procedure](../../scripts/acceptance/four-project-queue/RESOURCE-MEASUREMENT.md),
[recovery helpers](../../scripts/acceptance/launch-recovery/README.md),
[dependency-join preparation](../../scripts/acceptance/task-dependencies/README.md),
[checkpoint preparation](../../scripts/acceptance/task-continuation/README.md)
and [context-quality protocol](../../scripts/acceptance/context-quality/README.md).
Ignored cloud `artifacts/` and private fixture state are not required downloads.
**The Linux installed drivers require an exclusive test daemon. Do not run or
adapt their empty-daemon/cleanup routines against this shared Docker Desktop.**
Use the manual Windows matrix below. A helper's presence or pure tests do not
establish that its installed workflow passed.

For the actual source app, choose unused loopback ports. The launcher rechecks
them and refuses a conflict without attaching to or stopping another service:

```powershell
function Get-FreeLoopbackPort {
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    try { $listener.Start(); return $listener.LocalEndpoint.Port }
    finally { $listener.Stop() }
}
$apiPort = Get-FreeLoopbackPort
$webPort = Get-FreeLoopbackPort
while ($webPort -eq $apiPort) { $webPort = Get-FreeLoopbackPort }
.\scripts\start-local-acceptance.ps1 -AcceptanceRoot 'E:\Luci-local-acceptance\runs' `
    -Port $apiPort -FrontendPort $webPort
```

This creates a private fresh GUID run outside the checkout, with `state`,
`projects`, logs and `artifacts\windows-baseline.json`. It checks at least 40 GiB
on the selected volume, then builds/starts the API and locked frontend. It uses
process-local `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR` and
`LUCI_ADDR=127.0.0.1:<chosen API port>`, clearing inherited service/worker settings
and restoring them afterward. The free-port check is not a reservation; if a
race occupies a port, select another pair. `-PrepareOnly` records a baseline
without launch; a later invocation creates another fresh run.

Open the printed address; leave the launcher window open. Create the local
owner. If asked, read `<printed-run-folder>\state\control-plane\bootstrap-token`
privately and enter it into the app; never share the token. Set the actual Git
author identity in Settings and select **On this computer**. Ctrl+C requests
shutdown of this owned service and preserves its state/projects.

To reopen the same source workspace, use a fresh acceptance-only PowerShell
process and its retained manifest, not a new launcher invocation:

```powershell
$run = Get-Content '<printed-run-folder>\artifacts\windows-baseline.json' -Raw | ConvertFrom-Json
Get-ChildItem Env:LUCI_* | ForEach-Object { Remove-Item -LiteralPath ('Env:' + $_.Name) }
$env:LUCI_STATE_DIR = $run.stateDir
$env:LUCI_PROJECTS_DIR = $run.projectsDir
$env:LUCI_ADDR = $run.apiAddress
.\scripts\dev.ps1 -Port ([int]($run.apiAddress.Split(':')[-1])) `
    -FrontendPort $run.frontendPort -DevDirectory (Join-Path $run.artifactsDir 'dev')
```

## Storage, credentials and execution consent

Before **Install runtime**, the host Super Admin reviews the actual existing
VHD path in **Docker storage capacity**, confirms active and pending locations
are on mounted local drives, then explicitly confirms the reviewed location.
This operator confirmation does not claim automatic engine-to-VHD discovery or
move/edit Docker data. Team members/admins cannot read the private path or confirm
host storage. Use [WINDOWS-DOCKER-STORAGE.md](WINDOWS-DOCKER-STORAGE.md) for the
read-only probe, its `LUCI_WINDOWS_STORAGE_PROBE_VHD` and
`LUCI_WINDOWS_STORAGE_PROBE_REPORT` variables, native ACL and fresh pin checks.

Host physical free space and **Free inside Docker's Linux disk** are separate.
The physical observation is the minimum across all eligible mounted writable
local volumes, not only E:. Another low local volume can block new work; do not
hide/unmount it or weaken guards to pass. Runtime preparation requires fresh 40 GiB
on both sides (32 GiB working allowance plus 8 GiB reserve), an estimate rather
than a measured peak guarantee. Workers separately account for pending demand
and storage driver. Full local Supabase is optional and needs further memory,
CPU and post-install disk headroom; it is not implied by agent readiness.

The gateway produces a shared observation; workspace processes read its fresh
engine/confirmation-bound cache instead of each launching WSL probes. Missing,
stale or mismatched observations block new heavy work while preserving planning
and existing workers. Cached **Refresh Docker storage status** is distinct from
explicit **Check readiness**. Native919 verified the HTTP distinction, not exact
probe-start counts. WMI/CIM/ETW tracing was denied at that session's authority;
one-second samples cannot establish that brief `wsl.exe` processes did not start.
Record exact producer invocation counts with permitted tracing/instrumentation,
cache age/revision and multiple workspace process identities before claiming
native deduplication or its resource cost.

The current guest observation requires Docker Desktop WSL 2, its running
`docker-desktop` distribution and matching public engine identity. Hyper-V-only,
remote or unobservable layouts remain unsupported/unknown, with no unlimited
fallback. Do not create probe containers or alter global Docker settings merely
to get a measurement. Native confirmed file/ACL/cache scopes remain separate
from this candidate's actual readiness/admission behavior.

Configure credentials through Settings, never by copying host login files into
fixtures. [PROVIDERS.md](PROVIDERS.md) lists supported harnesses and exact limits:

- **Codex subscription:** official device sign-in, then save the completed
  sign-in. Current credentials permit one active session; do not call it a
  parallel-provider test or silently switch billing.
- **API connection:** explicitly select supported endpoint, harness and model,
  with provider concurrency and spending known. Prove a real small task first.
- **Local model:** configure an explicit compatible endpoint/model through
  OpenCode; the server must already run and be reachable from Docker. Container
  loopback is not Windows loopback. Include the model's real memory demand.

A saved badge/sign-in/health response is not task completion. Autonomous tasks
require the reviewed **Bypass** mode for the pinned single-turn CLI; Ask/Plan are
interactive alternatives and must not be silently upgraded. Verify Ask refuses
before allocation and an authorized task exits naturally. Record actual provider,
model, permissions and costs without secrets. An unattended queue waiting for
per-command native approval is not yet supported.

## Build and run the matching native app

Source checks can be run sequentially with `GOMAXPROCS=2` and `-p 1` in the
acceptance shell, retaining exact output/failures and restoring prior variables:

```powershell
.\scripts\test-dev-windows.ps1
if ($LASTEXITCODE -ne 0) { throw 'Launcher checks failed' }
go test -p 1 ./sandboxd/...
if ($LASTEXITCODE -ne 0) { throw 'Sandbox checks failed' }
go test -p 1 ./control-plane/...
if ($LASTEXITCODE -ne 0) { throw 'Control-plane checks failed' }
Push-Location desktop
try {
    npm.cmd ci
    if ($LASTEXITCODE -ne 0) { throw 'Locked install failed' }
    npm.cmd run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'Typecheck failed' }
    npm.cmd test
    if ($LASTEXITCODE -ne 0) { throw 'Frontend checks failed' }
    npm.cmd run build
    if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed' }
} finally { Pop-Location }
.\scripts\build-release.ps1 -Version 0.5.0 -Native
if ($LASTEXITCODE -ne 0) { throw 'Native build failed; retain its output.' }
```

For focused native controlled continuation checks, the exact affected prefix is
`go test -C control-plane -p 1 -count=1 -json ./internal/product ./internal/api -run '^TestSwarmCheckpoint'`.
It includes `TestSwarmCheckpointActualHTTPSettledLaunchPreparesThenExplicitlyStartsDistinctWorker`,
`TestSwarmCheckpointActualHTTPFinalizedFenceRequiresCurrentExactMarker`,
`TestSwarmCheckpointActualHTTPRechecksUnsettledAndRevokedAuthorityBeforeCopy` and
`TestSwarmCheckpointGuardedRoutesRefuseBusySourceBeforeNewWorkspace`. These use
real HTTP/Git/private state with controlled backend state. Keep skips, timeout
and failures distinct from passes; no native Docker/model claim follows. Follow
the separate recovery matrix for exact Windows lock/named-pipe checks.

The native build command has passed at the historical revisions above. It builds
the matching native app/sidecar/resources and bundled Linux runtime; it provisions
no cloud resource. Native NSIS generation is not installer execution. Build
without `-Native` for `artifacts/releases/0.5.0/luci-windows-amd64.zip`, a portable
browser/server distribution with `Start-Luci.cmd`; that is separate from native
WebView acceptance. See [RELEASE.md](RELEASE.md) for manifests and signing.

For personal-machine native smoke, use the verified built/extracted app with all
matching resources, **not the installer**. Never launch older native packages
`8f78804f`, `656a1ae1` or the incomplete `c46405fc` correction with an assumed safe
state override. Preserve normal roots `%APPDATA%\ai.lcfr.luci` and
`%LOCALAPPDATA%\Luci\state`: privately record absence or content hashes, file IDs,
creation/last-write times, length and attributes before/after. Refresh metadata
and use read-only native handles for discrepancies; access times are
observer-sensitive and must be recorded separately.

In a dedicated PowerShell process, use a fresh absolute, non-linked E: run folder:

```powershell
$nativeExe = '<verified-native-app-with-matching-resources>\luci.exe'
if (-not (Test-Path -LiteralPath $nativeExe -PathType Leaf)) { throw 'Select the verified native app.' }
$nativeRun = Join-Path 'E:\Luci-local-acceptance\native-runs' ([guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $nativeRun -ErrorAction Stop | Out-Null
Get-ChildItem Env:LUCI_* | ForEach-Object { Remove-Item -LiteralPath ('Env:' + $_.Name) }
$env:LUCI_STATE_DIR = Join-Path $nativeRun 'state'
$env:LUCI_PROJECTS_DIR = Join-Path $nativeRun 'projects'
$env:WEBVIEW2_USER_DATA_FOLDER = Join-Path $nativeRun 'webview-profile'
# LUCI_API remains absent: the native launcher owns this sidecar.
$app = Start-Process -FilePath $nativeExe -PassThru
$app.WaitForExit()
# Close this dedicated shell when finished; do not change user/system environment.
```

Use the local test bridge's `runtime_info` before owner setup. `stateDir` must
exactly match the selected absolute state, `startupError` must be empty, and
`apiUrl` is authoritative. Native own-sidecar mode chooses a free IPv4 loopback
port; inherited `LUCI_ADDR` does not select it. Record the exact app/sidecar/
WebView identities and normal exit. Reopen with the same three paths and same
native app to check persistence, returning sign-in and only-owned shutdown.
Invalid/relative/file/junction state targets must fail visibly without fallback.
Retain failed fixtures; do not delete private state to get a clean-looking result.

Deliberately setting `LUCI_API` to a known owned service tests **external mode**:
no new owned sidecar, local bootstrap read or external-service shutdown. Record
that separately from own-sidecar lifecycle. Source-browser and native WebView
outcomes are also separate.

**Do not run `scripts/test-installed-windows.ps1` on this ordinary machine or
forge runner environment variables.** It intentionally performs installation/
upgrade/uninstallation only in a disposable Windows runner. That gate needs a
separate disposable VM and retained backups, not a guard bypass.

## Integrated manual acceptance matrix

Use one fresh disposable project for destructive/conflict scenarios. Record
project/run/task/operation/session IDs, source and context revisions, actual
HTTP/results, preserved old bytes and exact cleanup. Source tests and prepared
helpers do not replace these matching-candidate workflows.

| Gate | Native app sequence and decisive result |
| --- | --- |
| Startup, owner and LOCAL-004/005/007 carry-forward | Confirm exact runtime/state; fresh owner and returning sign-in; four distinct saved purposes; labels select without expansion, disclosure/keyboard preserve focus, Ctrl+K and Keep editing work. Repeat affected cached-refresh navigation while a real response is held: one cached GET, no full check/install, no late reopening. Explicit readiness remains separate. Preserve normal state and original Docker workload identities. |
| CLOUD-010 older work stays actionable | Let run A finish with output awaiting review, then create newer run B. Home/orchestrator must retain A's exact project/run/task review or question link; opening it must not substitute B or count A as running capacity. Complete the decision and refresh/reopen; attention should reflect recorded state without losing the history. |
| CLOUD-011 reports, answers and V2 context | A stopped task writes its bounded structured handoff with decisions, limitations, claimed checks and a blocking question. Inspect attributed report and source/context receipt, answer through the app, then reserve a dependent task only after prerequisites are reviewed and integrated. Inspect its actual read-only V2 captured context and full direct-dependency packet. The old report/answer is frozen; later edits affect future reservations. Worker claims never appear as recorded passing checks or permissions. Missing/unsafe/unavailable reports remain explicit; a GET must not execute/capture/mutate work. |
| CLOUD-012 retained checkpoint | Produce committed, dirty and untracked ordinary text within the task scope, then stop/observe the exact launch (finish uncertain-launch recovery first if required). Open More task actions → Continue from retained changes; review complete before/after text and explicitly consent. A new ready reserved operation/child/workspace and seed appear; old workspace and primary remain unchanged, task/dependencies remain incomplete. Restart before start; same-key lost-response readback recovers the original reservation without recopying. Only a separate Continue reserved task starts the successor. Its context contains the exact prior report/answer and seed. Review final combined changes before integration. |
| Checkpoint refusal/interruption | Change retained source or an overlapping primary file after review; stale revision/digest must refuse before allocation. Revoked Edit/Execute, unsafe/binary/oversized/protected output and unresolved launch must refuse. An incomplete preparation remains unknown and is not automatically retried; explicit retention/cancellation of a proven never-started preparation cannot revive a delayed publisher. Generic Resume/Fork/Restart of the old reviewed session remains prohibited; do not use it as a continuation workaround. |
| CLOUD-013 dependency join and real Git conflict | Use one project with two prerequisites and a dependent join; record actual allowed concurrency. Before integration, joining must refuse. A deliberately stale review refuses without changing primary/index. Separately create a real overlapping Git merge conflict in this disposable project and retain both attempts and exact source state; this is not the same as stale-review refusal. Resolve through the supported review path. Only accepted integrations release the join, whose actual context names both exact prerequisites. Verify combined output independently. Serial execution can prove dependency ordering, not concurrent speed. |
| CLOUD-015 truthful integration outcome | Check the returned integration state, not HTTP 200 alone. A retained Git conflict/unknown integration must show unresolved work and preserved output, with no success/dependency-release notice. A confirmed integrated result may show success. A lost response must prompt observation of the exact attempt, not an automatic merge/retry or invented success. |
| CLOUD-016 terminal output budget | Paste a finite 100 KiB command into an owned interactive session and keep reading its output: fragmentation alone must not disconnect it. An actually stalled consumer exceeding the byte budget must still disconnect explicitly while other readers and the child continue. Do not infer bounded memory from only the healthy-reader case. |
| CLOUD-017 acknowledged manual start | Start one task through the app. Require the exact acknowledged child and automatic recorded status observation; do not press Check team status to manufacture the result. If observation fails, the acknowledged session must remain inspectable and the UI must explain uncertainty without replaying Start or silently starting the rest of a batch. |
| CLOUD-018 manual completion without queued work | With queue dispatch paused and no queued item, let a manually started task exit naturally with a structured report. Without refresh, navigation or a reconcile/collect action, its open card must become Awaiting review with the report/questions available. Home must then identify that exact build. Repeat observation across a normal service restart; report claims must not become recorded verification, integration or permission. Busy cleanup must retain capacity until independently confirmed, and uncertain disappearance must remain uncertain. |
| Recovery and no duplicate launch | Follow [LOCAL-RECOVERY-ACCEPTANCE.md](LOCAL-RECOVERY-ACCEPTANCE.md) and [the recovery contract](INTERRUPTED-LAUNCH-RECOVERY.md). Untouched reserve→retain/cancel→reopen→new reserve uses distinct identity. An uncertain started attempt cannot be replaced until exact protocol-supported recovery finishes. Actual Windows request death/lost ACK/durable finalization needs an isolated controlled fixture; do not kill unrelated user services or inject faults into the shared daemon. Legacy/unsupported/saturated history must say why and preserve output. |
| Useful result and amendment | Build a small app with explicit roles, persistence and phone/desktop journeys; run real functional checks, restart and verify saved data. Amend the idea, review changed context and affected scope, build/integrate it and rerun prior behavior. A natural CLI exit, reported check, screenshot or healthy preview is not functional acceptance. |

Checkpoint review currently supports complete Git-visible UTF-8 text only:
128 changed files, 128 KiB per changed file and 512 KiB total before/after text.
Ignored files are not copied; binaries, protected paths, links, ambiguous paths,
unsafe conversion rules and unsupported source drift refuse visibly. Disjoint
reviewed primary changes require exact affected-path base identity. The old
retained source never becomes integrated merely because it was adopted. Keep
unsupported cases visible instead of force-applying or silently omitting output.

## Sustained four-project capacity and provider quality

The reported workstation has **31.2 GiB physical RAM**. Take at least 60 seconds
of actual idle Luci with the normal browser/editor workload, then a **30-minute
native four-project workload** and a settle interval. Follow
[SWARM-CAPACITY-ACCEPTANCE.md](SWARM-CAPACITY-ACCEPTANCE.md) and the
[Windows sampler commands](../../scripts/acceptance/four-project-queue/RESOURCE-MEASUREMENT.md#native-windows-sampler-short-native-check-reported).
The sampler's nine controlled checks and 8.017-second native PowerShell/descendant
capture verify only their reported OS/ACL scope, not Luci performance or Docker
named-pipe sampling under sustained load.

Choose explicit host/global/per-project/account/provider limits from fresh
measurements. The default reserve derives from actual total RAM (25% with a 2 GiB
floor); record it rather than treating all 31.2 GiB as available. Four advancing
projects may share a smaller worker pool; report actual overlap and wait reasons.
Do not lower worker quality/resources silently or weaken storage/memory admission
because another application is using the machine. Record non-agent checks,
previews/data/dev servers as well as worker reservations and measured use.

Use distinct goals/decisions/task graphs and output witnesses, mix build/test/
review work, change priorities, pause/resume/cancel where supported, fail one
worker, amend context and restart the owned app with queued work. Each project
must progress without starvation, cross-project context leakage or duplicate
work. Keep previews and retained output usable and review exact changes.

Capture owned app/WebView/API/worker/test processes by PID **and creation
identity**; do not attribute every browser or `vmmemWSL` process to Luci. Keep
process working sets, Windows available physical RAM/commit, Docker cgroup usage,
limits and VM/shared-daemon overhead separate; sums double-count shared pages.
Measure CPU/I/O/disk pressure, actual sample gaps and per-project wait/active/check/
cleanup times. Measure UI input and project-switch latency plus ordinary editor/
browser responsiveness; counters or frame rates alone do not prove either.
Native exact short-lived process-start tracing remains unknown until permitted
instrumentation records it. Preserve missing measurements as unavailable.

Run an equivalent accepted workload serially and with bounded parallelism before
claiming speedup. Report retries, conflicts, failed checks and actual provider
usage/cost, or unavailable usage. Scripted providers prove mechanics only. Use
[the context-quality protocol](../../scripts/acceptance/context-quality/PROTOCOL.md)
for real ambiguity, isolation, amendments, stale source, handoffs and restart;
blind oracle checking/human review must judge actual delivered software, not the
agent's own success statement. Do not publish credentials or private transcripts.

## Evidence to return and open gates

Return the exact commit/package/image lineage, clean source/hash comparison,
selected provider/mode and limits, per-gate observed result, timings/resource
coverage, screenshots and sanitized HTTP/effect evidence, retained failure paths,
and exact owned cleanup plus preserved user workloads. Keep state, tokens,
credentials, raw private fixture paths and authentication logs local. Link the
clone's receipts and helpers; do not require inaccessible cloud artifacts.

The integrated source race run finished with 3,023 passing outcomes, 29 explicit
skips and one sandbox terminal failure; its 2,003 input hashes, HEAD and working
tree were unchanged. All control-plane packages passed. The terminal defect is
CLOUD-016: finite input can trigger a fragment-count output cap before its byte
budget is consumed. Its correction passes 32 affected hub/server race outcomes and remains separate
from the original failed full run. Full frontend checks recorded 2,575 passes and three
failures; TypeScript and the production build passed. Contract-mirror and recovery-test synchronization corrections pass within 169
affected frontend tests; TypeScript/build pass again. Final controlled browser
verification passes 22 desktop/phone checkpoints plus the original keyboard-only
checkpoint-focus reproduction. The backend follow-ups pass 126 affected
API/product race outcomes. See the [source receipt](evidence/2026-10-08-cloud010-015-source-verification.md).
Current C010/C011/C012/C013/C015 changes do not inherit older installed passes. Checkpoint controlled race checks have
passed 40 product and 8 real-HTTP API outcomes, but backend states were controlled:
no native Windows or installed two-worker claim follows. The earlier combined
run's separate C011 test-decoding assertion failure remains recorded; the full
control-plane run passed after the test fixture correction. Prepared continuation/dependency helpers and unfinished sustained
attempts remain open until matching installed runs complete.

Remaining gates include matching-candidate installed dependency/continuation/
conflict workflows, native Windows recovery and sustained whole-machine
responsiveness, permitted exact probe tracing, live-provider usefulness and
clean-host installer behavior. Historical Linux and native receipts remain valid
for their named scopes. These limitations do not wait on cloud billing or public
deployment, and they do not turn the local milestone into a completed product.
