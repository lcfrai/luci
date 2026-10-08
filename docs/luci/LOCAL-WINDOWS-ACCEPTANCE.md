# Local Windows handoff and acceptance

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

No hosted Luci account, Luci licence subscription, AWS/Azure account or cloud
credit is required. Choose your own model connection in the app. Provider API
usage or a model subscription may have its own costs. Existing host CLI login
files are not automatically imported into Luci.

## Get and run the source

Run these in a new working folder, leaving any existing checkout intact:

```powershell
git clone --branch codex/luci-production-runs https://github.com/lcfrai/luci.git luci-local
if ($LASTEXITCODE -ne 0) { throw 'Clone failed' }
Set-Location luci-local
git rev-parse HEAD
git status --short
go version
node --version
npm.cmd --version
docker version
docker info --format '{{.OSType}}'
```

The final Docker command must report `linux`. If the handoff specifies a
particular full commit, select that commit with `git switch --detach <commit>`
before continuing. Keep the printed commit with your results.

For an isolated acceptance workspace, set private state and project locations
outside the source checkout. Keep both folders for restart/recovery checks:

```powershell
$luciAcceptanceRoot = Join-Path $env:LOCALAPPDATA 'Luci-local-acceptance'
$env:LUCI_STATE_DIR = Join-Path $luciAcceptanceRoot 'state'
$env:LUCI_PROJECTS_DIR = Join-Path $luciAcceptanceRoot 'projects'
./scripts/dev.ps1
```

Open the browser address printed by the launcher and leave its PowerShell window
open. This route runs the actual source application in a browser; it is separate
from the native WebView installer gate. The launcher builds the API, waits for
readiness, installs locked frontend dependencies if absent and starts the client.
An occupied API port fails before startup; close the prior launcher or use
`./scripts/dev.ps1 -Port 8740`. The browser development server still requires
port 1420. The chosen API port is carried into its client automatically.

On first setup, create the installation's local owner. If the browser asks for
the first-run setup code, open the following private file on your own machine
and enter the code into the app. Do not paste it into an issue or a chat:

```powershell
notepad.exe (Join-Path $env:LUCI_STATE_DIR 'control-plane\bootstrap-token')
```

The file is consumed by successful first-owner creation. Set a real Git author
name/email in Settings, choose **On this computer**, use **Check readiness**, and
install the agent runtime using **Install runtime**. Alternatively the source
launcher can build the same runtime before opening the client:
`./scripts/dev.ps1 -InstallRuntime`. If installation fails, retain the failure,
resolve the reported Docker/network/disk issue and retry; a saved readiness
record alone does not establish a working engine.

Use Ctrl+C in the source launcher to stop it. Its API receives an owned shutdown
request, with forced cleanup limited to that process if necessary. State and
projects remain on disk. Relaunch with the same two environment settings to
resume the same acceptance workspace. Do not delete private state as a remedy
for an application failure.

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
| P0: Shared understanding | Describe a small facilities-maintenance app: staff report and comment on jobs; managers assign and change status; saved records survive restart; phone and desktop are usable. Review the proposed requirements, data/role rules, screens and acceptance checks. Resolve meaningful ambiguity and approve the agreed scope before execution. Show what is captured for the run and what remains a proposed amendment. |
| P0: Real parallel execution | Decompose at least two independent tasks and one task depending on both, with explicit acceptance criteria and integration boundaries. Start with a provider that permits concurrency. Prove independent tasks overlap in distinct worktrees/containers; the dependent task must wait for accepted prerequisites. Agents must write the actual application. Record task/run/session IDs, branches, source revisions and timestamps. Manual copy-in of a prebuilt app does not pass. |
| P0: Review and integration | Inspect each agent's diff and check results, deliberately create one conflicting edit, and review the resolution. Only approved changes enter the target branch. Stale source, dirty work, failed checks and unresolved dependencies cannot be labelled integrated or verified. Inspect the final combined diff and run the application's real checks. |
| P0: Intervention and recovery | Pause while independent work runs and verify the stated scheduling/process behavior. Steer an amendment deliberately; captured inputs for existing runs remain immutable. Cancel one agent, inspect its retained work and retry/resume through the app. Restart Luci during a run; recover truthful task/process state, context and next actions without duplicate agents or lost work. Do not treat a displayed stopped-session fixture as a live recovery check. |
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
unresolved failure, described in [FULL-PRODUCT.md](FULL-PRODUCT.md). No fresh
Windows pass or completed real-model swarm journey is claimed here. The exact
new implementation's source/runtime receipts belong in the accompanying handoff;
historical package results are not automatically results for this revision.

Finish all available local source/runtime verification before handoff. Dylan's
Windows and provider tests close only the gates they actually exercise. Keep the
local milestone open until its required gates pass. After that local feedback
and supplied access, use Luci's own reviewed agent workflows on Luci's source to
implement and verify cloud connectivity/execution. Public cloud operation,
managed billing and public distribution retain their separate release gates.
