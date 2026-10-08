# Local Windows handoff and acceptance

**Integrated handoff preparation — final revision pending.** This document is
not a readiness announcement. The next handoff must supply a full retrievable
commit after the storage-confirmation/shared-observation and CLOUD-008 changes
are integrated and checked. Do not substitute the moving branch tip. The
verified `2819ce9e` scopes below remain evidence for that earlier build; they do
not verify later changes or finish the local product.

First native acceptance is performed by the originating local testing thread
`01a1199f-7500-7843-b02b-081081e536fa` on host `local`, using an isolated checkout
and preserving the owner's existing changes. The cloud implementation thread
will send the exact tested revision and handoff when ready, or post
**LOCAL TEST HANDOFF READY** if thread messaging is unavailable. No cloud agent
should start processes on the owner's machine as part of this handoff.

Follow [LOCAL-TEST-LOOP.md](LOCAL-TEST-LOOP.md) for the continuous findings/fix
handoff protocol and current machine preparation observations. Use the spacious
E: or F: volume for an isolated checkout and caches; separately verify Docker's
data-volume capacity. Keep existing C: projects and installed application state.

Acceptance includes actual shadcn/ui styled interactive controls and scoped
cross-project actions: delegate, prioritize, inspect, pause/resume/cancel/retry
where supported, review changes and verified outputs. Multi-select/bulk actions
must state their scope and show partial failures without silently retrying work.

## Sustained four-project acceptance is required

The owner's machine reports **31.2 GiB physical RAM**. Measure its idle baseline
with normal work applications open before choosing a Luci budget; leave room for
those applications. Four projects advancing does not mean four costly agents
must run at once. Global queueing, fair progress and useful intervention are part
of local acceptance, alongside the single-project workflow below.

Run four distinct projects with separate goals, decisions, graphs and output
witnesses. Mix implementation, build/tests and review; switch among them, change
a priority, cancel/fail one worker, and restart Luci during queued work. Verify
that each project progresses, only its context is delivered, no attempt is
duplicated, outputs remain available, and changed source requires renewed review.
Track all owned service/agent/test/dev-server/browser processes and containers,
host available RAM, peak/idle usage, growth, CPU/disk pressure and UI latency.
Compare useful checked results and waiting time per project, not process counts.
Record the baseline, chosen budgets and workload duration with raw measurements.
Native Windows and live-provider results must be observed on this machine; Linux
fixtures and cross-compilation cannot close those gates.

The immediate milestone is a dependable local application for taking an idea
through coordinated agents to a reviewed, working result. Windows is the owner's
test machine. Cloud accounts, managed-team billing, public DNS and hosted
infrastructure are later gates; none is required for this local milestone.
A running UI, mocked provider, cross-compiled executable or package build does
not pass the real Windows, model or swarm workflow gates below.

Use the exact revision in the handoff on branch `codex/luci-production-runs`
([PR #6](https://github.com/lcfrai/luci/pull/6)). Record its full commit before
testing. This document travels with the revision; it does not label the moving
branch or an older downloadable installer as a verified release.

## Prerequisites

Use Windows x64 with a currently supported Docker Desktop configuration and
Linux containers. The normal browser source route needs Git, Go **1.26.3**, Node
**24.19.0**, a current Edge or Chrome browser, and Docker Desktop with its engine
running. Use PowerShell 7 for the commands below. The source launcher itself also
supports Windows PowerShell 5.1. Native Tauri packaging additionally needs Python
**3.11+**, stable Rust with the `x86_64-pc-windows-msvc` target, Visual Studio C++
build tools/Windows SDK and Microsoft Edge WebView2. Follow the official
[Tauri Windows prerequisites](https://v2.tauri.app/start/prerequisites/).

The local tester has reported the native prerequisites present: Rust/cargo
1.99 with the MSVC target, VS Build Tools 2019 16.11.53/VC 14.29.30133, Windows
SDK 10.0.19041.0, WebView2 154.0.4258.62 and Python 3.13.5/3.11.15. Conflicting
native64/WOW6432Node SDK registry discovery is an **unproven packaging concern**.
Do not reinstall or change global SDK registration preemptively; retain a real
packaging failure and resolve supported discovery only if it occurs. Presence
of these tools does not establish a successful native package.

Docker is required for isolated agent execution, checks and container previews.
Defining and designing a project remain available before Docker is ready. The
first agent-runtime installation downloads base images and the pinned CLI tools;
it is not an offline installation. Allow memory and disk for the actual number
of concurrent agents and the application being built. Full local Supabase is
optional for an application that does not use it; the stack requires another
4 GiB RAM and 2 CPUs beyond other reserved workloads, plus 40 GiB free disk
**after** image installation. For combined data/app/agent acceptance, plan for at
least 8 GiB RAM and 4 CPUs and verify the actual remaining headroom. Do not confuse
Docker Desktop's Linux VM capacity with the Windows drive's free space.

The first local preflight found Docker Desktop stopped and its existing VHD on
C:, with only 4.22 GiB free on that drive, despite ample E:/F: capacity. The
latest local report has Docker Desktop 4.48.0 (207573), Linux engine 28.5.1 and
the existing inventory running, with the supported relocation still awaiting
Windows unlock and C: last reported at 3.49 GiB free. A cold backup on E: has
been hash-verified. These are historical observations, tracked as
`LOCAL-ENV-001` in [LOCAL-TEST-LOOP.md](LOCAL-TEST-LOOP.md). Before any
image pull, runtime build or agent workload, confirm Docker Desktop's actual
disk-image location and record free space on that physical backing volume as
well as in the Linux engine. Recheck these observations rather than treating the
earlier numbers as current capacity. A VHD's file length and the VM's `df` output
are not substitutes for host-volume free space.

Read-only native checks include `Get-Volume -DriveLetter C,E,F` and inspecting
the configured image location in Docker Desktop's settings. Keep the existing
image and application data intact. If relocation is selected, use Docker
Desktop's supported disk-image-location workflow with an explicit local plan;
do not manually move/delete its live VHD or unregister WSL distributions.
The local tester owns this preparation. This cloud task does not initiate changes
on the owner's machine or infer that launching the engine resolves disk pressure.

No hosted Luci account, Luci licence subscription, AWS/Azure account or cloud
credit is required. Choose your own model connection in the app. Provider API
usage or a model subscription may have its own costs. Existing host CLI login
files are not automatically imported into Luci.

## Get and run the source

The local tester already uses `E:\Luci-local-acceptance`. Prepare a new checkout
there for the next pinned batch; preserve earlier checkouts and evidence,
including the contaminated `9898cf5d` diagnostic checkout. Replace the pending
value only with the final full commit supplied by the implementation thread:

```powershell
$candidate = 'PENDING_FULL_COMMIT_FROM_HANDOFF'
if ($candidate -notmatch '^[0-9a-f]{40}$') { throw 'Wait for the exact integrated handoff commit.' }
$checkout = Join-Path 'E:\Luci-local-acceptance\checkouts' $candidate.Substring(0, 8)
if (Test-Path -LiteralPath $checkout) { throw 'Choose a new checkout; preserve prior evidence.' }
git clone --branch codex/luci-production-runs https://github.com/lcfrai/luci.git $checkout
if ($LASTEXITCODE -ne 0) { throw 'Clone failed' }
Set-Location $checkout
git switch --detach $candidate
if ($LASTEXITCODE -ne 0) { throw 'The handoff revision could not be selected.' }
if ((git rev-parse HEAD).Trim() -ne $candidate) { throw 'Revision mismatch' }
if (@(git status --porcelain).Count -ne 0) { throw 'Use a clean candidate checkout.' }
go version
node --version
npm.cmd --version
docker version
docker info --format '{{.OSType}}'
```

The final Docker command must report `linux`. Keep the full commit and a tracked
file-hash snapshot with each batch, then compare them afterward. Use the local
thread's task-scoped E: tool activation, caches and TEMP; do not change system
Node or existing application configuration.

The clone includes [sanitized evidence](evidence/2819ce9e/README.md), the
[installed workflow helpers](../../scripts/acceptance/four-project-queue/README.md),
the Windows resource sampler and its C# companion, and the safe launcher. The
Linux Docker workflow driver is not a Windows driver. Follow the native sampler
procedure and interactive sequence below on Windows; ignored cloud `artifacts/`
directories and private fixture state are not required downloads.

For an isolated acceptance workspace, use the roomy E: or F: drive confirmed in
the local testing thread (substitute the actual chosen drive below):

```powershell
.\scripts\start-local-acceptance.ps1 -AcceptanceRoot 'E:\Luci-local-acceptance\runs'
```

Open the browser address printed by the launcher and leave its PowerShell window
open. This route runs the actual source application in a browser; it is separate
from the native WebView installer gate. The launcher builds the API, waits for
readiness, installs locked frontend dependencies if absent and starts the client.
The acceptance launcher creates a new private GUID-named folder containing its
state, projects, build/log outputs and a native Windows baseline. It requires
40 GiB free on that selected volume and uses API port 18730 and frontend port
14200 by default. Occupied ports fail without stopping an existing service;
choose explicit `-Port` and `-FrontendPort` alternatives if needed. It temporarily
clears inherited Luci service settings and restores them when the run ends.
`-PrepareOnly` records a baseline without starting the app; each invocation
creates a new run. The local tester reported the isolated launcher and source
browser working at `2819ce9e`; this does not verify the new storage probe,
an installed WebView package, or a later candidate.

On first setup, create the installation's local owner. If the browser asks for
the first-run setup code, open the following private file on your own machine
and enter the code into the app. Do not paste it into an issue or a chat:

```powershell
notepad.exe '<printed-run-folder>\state\control-plane\bootstrap-token'
```

The file is consumed by successful first-owner creation. Set a real Git author
name/email in Settings and choose **On this computer**. Complete the host
storage review below before choosing **Install runtime**. Keep dispatch paused
while configuring storage if any work was previously authorized. Use the app's
runtime preparation action so its storage preflight applies; do not use the
direct `dev.ps1 -InstallRuntime` build as an admission workaround. If installation fails, retain the failure,
resolve the reported Docker/network/disk issue and retry; a saved readiness
record alone does not establish a working engine.

Use Ctrl+C in the source launcher to stop it. Its API receives an owned shutdown
request, with forced cleanup limited to that process if necessary. State and
projects remain on disk. To resume the **same** workspace after stopping it,
open a fresh PowerShell window in the checkout and load that run's manifest:

```powershell
$run = Get-Content '<printed-run-folder>\artifacts\windows-baseline.json' -Raw | ConvertFrom-Json
# This is a fresh acceptance-only PowerShell process. Do not alter user/system settings.
Get-ChildItem Env:LUCI_* | ForEach-Object { Remove-Item -LiteralPath ('Env:' + $_.Name) }
$env:LUCI_STATE_DIR = $run.stateDir
$env:LUCI_PROJECTS_DIR = $run.projectsDir
$env:LUCI_ADDR = $run.apiAddress
.\scripts\dev.ps1 -Port ([int]($run.apiAddress.Split(':')[-1])) -FrontendPort $run.frontendPort -DevDirectory (Join-Path $run.artifactsDir 'dev')
```

Do not delete private state as a remedy for an application failure.

### Windows capacity requirements

The upcoming candidate adds **Docker storage capacity** in the host Super
Admin's Settings. A team member or team administrator cannot confirm host
storage. Enter the existing active VHD's full path only after checking Docker
Desktop's active and any pending disk locations. Acknowledge that they are on
mounted local drives, choose **Review storage location**, inspect the summary,
then **Confirm Docker storage location**. Luci does not move the VHD, change
Docker settings, or discover the active backing path from a guessed JSON key.
This is an explicit operator confirmation, not an automatic proof that a
particular VHD backs the engine.

The confirmation is revisioned in private host state. A fresh shared gateway
observation must match the selected engine/version, operator/file identity and
settings before new heavy work is admitted. Managed workspace workers read
that observation from the host scope; they do not each launch WSL or enumerate
Windows volumes. **Refresh Docker storage status** reads cached status and does
not launch a native probe. Background observation and explicit confirmation
perform the read-only native checks. Planning remains usable while storage is
unknown; existing workers are not killed because a new measurement fails.

The two displayed quantities are deliberately separate: **Free inside Docker's
Linux disk** and **Lowest free space across mounted local drives**. The latter
is a conservative minimum across all eligible mounted writable local volumes,
not free space on just the VHD's drive. Low C: space can therefore still block
runtime preparation after a supported move to E:. Do not hide/unmount an
unrelated drive or reduce a guard to pass acceptance. Runtime preparation
currently requires a fresh 40 GiB estimate on both sides (32 GiB build allowance
plus 8 GiB reserve); this is not a measured build-size guarantee. Worker starts
also account for pending allocations and the current image/storage driver.
Confirmation alone, stale samples, an engine mismatch or unreadable shared
state never establish capacity.

Before a workload, run the dedicated opt-in
[read-only native probe](WINDOWS-DOCKER-STORAGE.md#read-only-native-probe-acceptance)
from the final pinned checkout's `control-plane` directory. Its required names
are `LUCI_WINDOWS_STORAGE_PROBE_VHD` (the actual existing file selected by the
operator) and `LUCI_WINDOWS_STORAGE_PROBE_REPORT` (a fresh private report path).
The command is
`go test -count=1 -run '^TestWindowsLocalStorageReadOnlyProbe$' -v ./internal/resourcebudget`.
Follow the linked `try/finally` procedure to clear only those task-specific
variables afterward. It reads file attributes, settings fingerprint and mounted
volume capacities; it neither contacts Docker nor saves a host confirmation.
A skipped probe is unverified. This does not replace testing the actual Settings
confirmation, shared gateway cache, role denial and admission behavior.

The automatic daemon-storage probe currently requires Docker Desktop's **WSL 2
Linux engine** and its running `docker-desktop` distribution. It invokes the
Windows system `wsl.exe`, reads the public engine ID in the matching `dockerd`
mount namespace, and measures that filesystem. It does not create a probe
container, pull an image or infer VM disk space from Windows drive space.
Use `wsl.exe --status`, `wsl.exe --list --verbose` and
`docker.exe info --format '{{.OSType}} {{.OperatingSystem}}'` to record the setup.
Hyper-V-only, remote Docker, an inaccessible WSL distribution, or an engine
without the matching public `engine-id` file leaves execution storage unknown
and new heavy work waiting. Select Docker Desktop's WSL 2 engine and retry its
readiness check; no unlimited-capacity fallback is applied. Native compatibility
of this probe remains an acceptance gate.

The installation's account session cap and the global scheduler's capacity
policy both apply. Fresh account policy currently permits one session per user;
choose an explicit higher cap for simultaneous-agent acceptance and record it.
The Codex subscription connection still permits one active session. Use an
explicitly configured API/local-model connection with sufficient concurrency to
test truly parallel agents. Physical available RAM, Windows commitment, Docker
allocation, worker hard limits and observed usage are separate measurements;
Docker's allocation is never presented as observed free VM RAM.

## Connect a real model

In Settings, choose one connection and record its name, harness, model and
billing mode without recording its secret:

- **Codex subscription:** choose **Open Codex account sign-in**, complete the
  official device flow in the Docker terminal, then choose **Save Codex sign-in**
  after that session exits. Run a small implementation task to establish actual
  entitlement and model execution. Current retained subscription credentials
  admit one active session at a time; do not use that mode to claim parallel
  swarm acceptance or silently switch to API billing.
- **API access:** save your supported OpenAI/Anthropic or explicit compatible
  endpoint credentials through Settings, select the matching harness/model and
  complete a small real task. This is the appropriate parallel acceptance route
  where the provider permits concurrent requests.
- **Local model:** configure an explicit OpenAI-compatible endpoint and model
  through OpenCode. The server/model must already be running and reachable from
  Docker; container loopback is not Windows loopback. Luci translates configured
  host-loopback endpoints for its containers. Verify actual connectivity and
  sufficient model capacity before measuring parallel work.

See [PROVIDERS.md](PROVIDERS.md) for precise harness support and restart/sign-in
limits. A configured badge, completed account sign-in or successful health probe
is not evidence of a model completing a task. Verify disconnect/reconnect errors
remain actionable and never silently change model, provider or billing account.

### Select the task approval model deliberately

Autonomous coordinated tasks use a native single-turn command that exits when
the turn ends. The pinned CLI modes support this with explicitly reviewed
**Bypass** permission, meaning commands run without per-command approval inside
the isolated Docker workspace. Ask and Plan remain available through interactive
agents; they must not be silently upgraded to run unattended. The task screen
explains the saved choice before preparation. Change it through plan review and
a new build, not by rewriting an already consented task.

Test both paths: an Ask task is refused before any reservation or container is
created, and an explicitly authorized autonomous task ends naturally, releases
capacity and lets a different queued project progress without pressing Stop.
Review retained changes and run checks before integration; an ended process
alone does not establish a correct result. Record actual behavior for the chosen
live provider, including command refusal, incomplete output and failed checks.
An unattended queue that can pause for individual native approval requests is
not yet supported; see [PROVIDERS.md](PROVIDERS.md).

## Source checks and optional packages

From the checkout root, these commands run engineering checks. Failures remain
failures; none can be replaced by a visual launch check:

```powershell
./scripts/test-dev-windows.ps1
if ($LASTEXITCODE -ne 0) { throw 'Launcher checks failed' }
go test ./sandboxd/...
if ($LASTEXITCODE -ne 0) { throw 'Sandbox tests failed' }
go test ./control-plane/...
if ($LASTEXITCODE -ne 0) { throw 'Control-plane tests failed' }
Push-Location desktop
try {
    npm.cmd ci
    if ($LASTEXITCODE -ne 0) { throw 'Dependency installation failed' }
    npm.cmd run typecheck
    if ($LASTEXITCODE -ne 0) { throw 'Type checking failed' }
    npm.cmd test
    if ($LASTEXITCODE -ne 0) { throw 'Frontend tests failed' }
    npm.cmd run build
    if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed' }
} finally { Pop-Location }
```

To build a portable Windows application, run
`./scripts/build-release.ps1 -Version 0.5.0` from PowerShell 7 after installing
desktop dependencies and Python. The script also builds the matching bundled
Linux runtime; building that archive provisions no cloud resource. The portable
package is `artifacts/releases/0.5.0/luci-windows-amd64.zip`. Extract the whole ZIP
into a fresh folder and run its `Start-Luci.cmd`; do not copy just `luci.exe`.
Check its hash against `SHA256SUMS` and record `release-manifest.json`, which
must identify the tested commit and whether source changes were present.
Portable runtime use does not need host Go, Node or Python.

With the native prerequisites installed, use
`./scripts/build-release.ps1 -Version 0.5.0 -Native`. The unsigned NSIS installer
is under `desktop/src-tauri/target/release/bundle/nsis/`; install and launch it
normally as your user. Check startup, close/reopen, account and project
persistence, WebView keyboard/zoom and the same real workflow below. Native and
portable launchers can select different state locations; confirm the displayed
workspace before interpreting an empty installation as data loss. Follow
[RELEASE.md](RELEASE.md) for state migration and package provenance.

Do **not** run `scripts/test-installed-windows.ps1` on your personal machine or
set fake GitHub runner variables to bypass its guard. It performs install and
uninstall acceptance in a disposable Windows GitHub runner and intentionally
refuses ordinary machines. A clean-host install/upgrade/uninstall check belongs
in a disposable Windows VM, with the original candidate and state backed up.

## Prioritized local acceptance

Record each gate as passed, failed or unverified with the revision, environment,
actual actions and result. An unavailable control is a failed or blocked gate,
not permission to substitute terminal work and call the application complete.

| Priority / gate | Reproducible acceptance and required evidence |
| --- | --- |
| P0: Startup and configuration | Fresh source or matching package starts; owner creation and returning sign-in work; missing/stopped Docker has an actionable remedy; runtime installation succeeds; real selected provider completes a task. Repeat launch with an occupied port and a stopped engine; retain the truthful failures and recovery. |
| P0: Physical storage and host authority | In host Super Admin Settings, review and confirm the actual active/pending local layout. Record fresh, separate host-minimum and engine-guest values. Missing, stale, changed or unreadable observations block new preparation/worker starts; members cannot view the private path or confirm it. Refresh/status polling must not launch native probes or Docker work. Restart with the same private state and verify workers consume the gateway observation. Keep this gate open while relocation or sufficient space is unresolved. |
| P0: Four-project operator flow | Follow the mixed-workload sequence in [SWARM-CAPACITY-ACCEPTANCE.md](SWARM-CAPACITY-ACCEPTANCE.md): choose projects, create/delegate scoped work, distinguish queued/running/blocked/review states, prioritize, pause/resume/cancel/retry where supported, and review exact changes. Bulk selection must show scope, cleared/stale selections and per-item failure. No project must disappear, inherit another project's context, starve or silently duplicate work. |
| P0: Shared understanding | Describe a small facilities-maintenance app: staff report and comment on jobs; managers assign and change status; saved records survive restart; phone and desktop are usable. Review the proposed requirements, data/role rules, screens and acceptance checks. Resolve meaningful ambiguity and approve the agreed scope before execution. Show what is captured for the run and what remains a proposed amendment. |
| P0: Real parallel execution | Decompose at least two independent tasks and one task depending on both, with explicit acceptance criteria and integration boundaries. Start with a provider that permits concurrency. Prove independent tasks overlap in distinct worktrees/containers; the dependent task must wait for accepted prerequisites. Agents must write the actual application. Record task/run/session IDs, branches, source revisions and timestamps. Manual copy-in of a prebuilt app does not pass. |
| P0: Review and integration | Inspect each agent's diff and check results, deliberately create one conflicting edit, and review the resolution. Only approved changes enter the target branch. Stale source, dirty work, failed checks and unresolved dependencies cannot be labelled integrated or verified. Inspect the final combined diff and run the application's real checks. |
| P0: Intervention and recovery | Pause while independent work runs and verify the stated scheduling/process behavior. Steer an amendment deliberately; captured inputs for existing runs remain immutable. Cancel one agent, inspect its retained work and retry/resume through the app. Restart Luci during a run; recover truthful task/process state, context and next actions without duplicate agents or lost work. Do not treat a displayed stopped-session fixture as a live recovery check. |
| P0: Natural completion and cleanup retry | A completed native task exits without Stop/DELETE and preserves its output awaiting review. Record exit observation, verified container absence, next-project reservation/create/start and integration times. CLOUD-008 must release capacity after proven cleanup in the same sweep, not after another fixed reaper interval. A failed removal retains the reservation and retries; unknown/running/replaced containers and missing/corrupt lease proof cannot be deleted or marked clean. Controlled failure injection belongs in the tracked scoped tests, never by manipulating an unrelated local container. |
| P0: Working local result | Run the built application through Luci, exercise staff/manager permissions and the agreed journeys, restart the app and confirm durable records. Make an amendment, build and review it, then recheck the existing data and previous behavior. A green process exit, screenshot or preview health response alone does not pass functional acceptance. |
| P1: Context and handoffs | Inspect each task's assigned context, captured requirements/design/plan and prior accepted outcomes. Compare an active run against a later reviewed amendment. Resume a stopped session and verify its captured context remains correct. A replacement agent must understand completed work, unresolved questions, verification evidence and the next task without relying on an unbounded chat transcript. |
| P1: Efficiency and quality | Run the same approved workload from the same starting commit with concurrency one, then with a bounded parallel configuration. Record preparation, active, waiting, verification and total wall time; peak active agents; available memory/CPU data; actual reported tokens/cost or explicitly unavailable usage; retries, conflicts and failed checks. Report speedup only for equally accepted outcomes. Parallel activity is not itself evidence of a faster or better result. |
| P1: Operator polish | During real activity, identify the current phase, task owner, dependency/blocker, running work, next decision and exact built scope without reading logs. Exercise keyboard navigation, phone-width view, 400% zoom, unsent-text preservation and long failure messages. Pause/cancel/retry/review must have clear consequences and trustworthy feedback. |
| P1: Native Windows lifecycle | Install the matching candidate, launch its own sidecar, reopen and recover state, complete the workflow in WebView, and verify clean close. In a disposable VM, perform upgrade/uninstall with retained project/private-state hashes; do not infer this from a Linux build or successful cross-compilation. |

Retain a small sanitized report with failed attempts, source/package hashes,
actual provider/model, hardware and limits, task timings, resulting commit,
commands with exit status, application behavior and recovery observations.
Keep credentials, private conversation/state files and raw provider request logs
out of reports. Record unsupported or unverified behavior explicitly.

## Current evidence and remaining limits

At exact `2819ce9eacf62b2d73a1522b2db25301a7b393d4`:

- [Installed Linux mechanics](evidence/2026-10-08-2819ce9e-installed-swarm.md)
  passed four actual Docker/native tasks across four projects, one worker at a
  time, with natural exit 0, live-child recovery after service restart, immutable
  task context, cancellation before allocation and reviewed integration.
  Scripted provider replies are not live-model quality evidence. The 428.478 s
  workflow retained 57.521–65.457 s completion-poll-to-next-create delays; this is
  the CLOUD-008 baseline, not a verified speed improvement.
- [Native correction checks](evidence/2026-10-08-2819ce9e-native-fixes-reported.json)
  were reported passing by the local tester: nine selected tests, unchanged
  HEAD and all 1,694 tracked-file hashes. LOCAL-001/002/003 are verified for
  their original reproductions; the earlier failed/mutated checkout is retained.
- [Native source-browser smoke](evidence/2026-10-08-2819ce9e-native-browser-reported.json)
  was reported passing nine checks: isolated launch/owner setup, four saved
  project contexts, scoped drafts, switching and service/browser restart.
  Home had zero axe violations and no 800 px horizontal overflow. No Docker
  work or live model ran, and these reports were not independently rerun by cloud.
- [Package/source receipts](evidence/2819ce9e/README.md) bind 996 installed
  members, 347 licence entries and the clean source export. The
  [resource observations](evidence/2026-10-08-2819ce9e-installed-swarm-resources.md)
  retain 445 samples over 450.37 s, with a 1.28 GiB peak complete observed
  driver-family RSS sum. This is a shared Linux host, not Windows capacity,
  an idle baseline, a 30-minute soak or input-latency evidence.

The next handoff must add its own exact source/package identity and changed-scope
checks for storage confirmation/shared gateway observation and CLOUD-008
same-sweep cleanup/retry. Do not transfer the old installed pass to those changes.

The Windows source launcher has ten executable control-flow checks covering
startup exit, delayed readiness, bounded timeout, occupied port, unexpected
service, dependency/build/runtime/frontend failure and owned forced cleanup.
They execute the real launcher under PowerShell 7.5.5 on Linux with controlled
external commands and a real loopback socket; they do not prove Windows process,
Docker Desktop or native WebView behavior. Its result is retained at
`artifacts/windows-source-launcher/results.json` in the development workspace.

Read-only CI inspection found run
[37543084295](https://github.com/lcfrai/luci/actions/runs/37543084295), commit
`1411294aead059907bb0f3898f6ecb2cf3c8e55f`, failed before any job steps executed;
the Windows job was skipped. Earlier installed Windows acceptance also has an
unresolved failure, described in [FULL-PRODUCT.md](FULL-PRODUCT.md). The focused
native results above do not close installed Windows acceptance or a completed
real-model swarm journey. The exact
new implementation's source/runtime receipts belong in the accompanying handoff;
historical package results are not automatically results for this revision.

Finish all available local source/runtime verification before handoff. Dylan's
Windows and provider tests close only the gates they actually exercise. Keep the
local milestone open until its required gates pass. After that local feedback
and supplied access, use Luci's own reviewed agent workflows on Luci's source to
implement and verify cloud connectivity/execution. Public cloud operation,
managed billing and public distribution retain their separate release gates.
