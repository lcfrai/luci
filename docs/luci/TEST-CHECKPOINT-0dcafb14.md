# Windows source test checkpoint — 0dcafb14

**Useful test checkpoint; local release acceptance is still open.** Source is publicly retrievable at [`0dcafb14da7d1f7e2eedaf70366f1e3cf4f4303f`](https://github.com/lcfrai/luci/commit/0dcafb14da7d1f7e2eedaf70366f1e3cf4f4303f), on `codex/luci-production-runs`. [Package receipts and reproduction helpers](https://github.com/lcfrai/luci/tree/7aeb3373f9cf21c1c8d39711f9ee6ec294226633/docs/luci/evidence/cloud042-package-0dcafb14) are published at `7aeb3373`. Linux archives remain private cloud artifacts, **not downloadable releases**; use the exact source to build Windows.

Clean Linux packaging matched **953 files, 347 licensed components, 44 guides and 203 revision-bound hosted URLs**. Portable archive: **69,000,975 bytes**, SHA256 `501a29e3c920173b99041797f794564b433e9d9b6c45036509165a4092a00b25`. Embedded cloud archive: **34,595,815 bytes**, SHA256 `0fd56b73af03bcf074de420c631068f2e1519757bb6d9fced499df0f49a200c6`. These identify Linux artifacts; a Windows build has its own manifest/checksums. Archive reduction is not a RAM or latency result.

Matching **scripted-provider/native-Codex** [Linux sustained attempt 6](https://github.com/lcfrai/luci/tree/7aeb3373f9cf21c1c8d39711f9ee6ec294226633/docs/luci/evidence/sustained-0dcafb14-attempt6) **failed** (zero external model calls): three natural task exits, two reviewed integrations; B refused `cpu_pressure` before Docker allocation. Idle/loaded switch p95 **485.37/551.56 ms** fail 300 ms; input **64.61/72.25 ms** passes 100 ms. All 1,292 actions succeeded, but workflow/settled-phase acceptance did not. Closure records empty Docker/volumes/ledger, 703 unchanged fixture files, 13 absent known identities and **two exited Chromium zombies**, with three unrelated environments unreadable. CLOUD-045 terminal-refusal reporting and CLOUD-046 admission diagnostics are in progress and **not part of 0dc**; the next revision will identify them separately.

Native 0dc packaging/runtime, real-provider quality, remaining recovery cases and sustained four-project performance remain unverified. Prior Windows passes retain their original revision/scope. This checkpoint does not request a workload while the local RAM gate is closed.

## Preserve the machine; prepare only when resources permit

Keep the original C: checkout, normal app data, existing E: evidence and all unrelated applications/workloads. Use the tester's task-local **Node 24.19.0/npm 10.9.3, Go 1.26.3**, E: caches/TEMP/TMP; native packaging also needs **PowerShell 7, Python, Rust/cargo 1.99 MSVC, C++ Build Tools/Windows SDK and WebView2**. Reuse established activation; do not modify global PATH or SDK settings. Measure fresh available RAM/commit and actual Docker guest/backing-volume space before each batch; do not relax admission or stop ordinary apps to fit it.

Docker's relocated E: disk and existing 107-container/four-running baseline must be preserved by exact identities. **Do not run Linux exclusive/empty-daemon drivers on this shared Docker Desktop or bypass `test-installed-windows.ps1`'s disposable-runner guard.** See [full Windows procedure](LOCAL-WINDOWS-ACCEPTANCE.md) and [two-way test loop](LOCAL-TEST-LOOP.md); this checkpoint supplies the newer exact source identity.

## Retrieve a fresh isolated checkout

The local tester already has a clean isolated `E:\Luci-local-acceptance\checkouts\0dcafb14` and a successful prepare-only baseline. Reuse that verified preparation between closed batches; do not repeat it solely for this document. For a new test checkout, run in the prepared task-local PowerShell shell after its resource gate permits:

```powershell
$ErrorActionPreference = 'Stop'
$candidate = '0dcafb14da7d1f7e2eedaf70366f1e3cf4f4303f'
$checkout = Join-Path 'E:\Luci-local-acceptance\checkouts' ('0dcafb14-' + [guid]::NewGuid().ToString('N'))
git clone --no-checkout https://github.com/lcfrai/luci.git $checkout
if ($LASTEXITCODE -ne 0) { throw 'Clone failed' }
git -C $checkout fetch origin codex/luci-production-runs
if ($LASTEXITCODE -ne 0) { throw 'Fetch failed' }
git -C $checkout checkout --detach $candidate
if ($LASTEXITCODE -ne 0) { throw 'Pinned source unavailable' }
Set-Location $checkout
if ((git rev-parse HEAD).Trim() -cne $candidate -or @(git status --porcelain).Count) { throw 'Require exact clean source' }
```

Keep normal remote-tracking provenance; do not synthesize refs to force compact packaging. Record HEAD/source hashes before and after. Change revisions only between closed test batches.

## Source launch and matching native build

For an initial source UI batch, select distinct free loopback ports; the launcher rechecks them and creates private fresh state/projects/evidence on E:. It does not request Docker runtime installation. Settings actions can install runtime, so keep that separate until its resource gate permits.

```powershell
function Get-FreeLoopbackPort {
  $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
  try { $listener.Start(); return $listener.LocalEndpoint.Port } finally { $listener.Stop() }
}
$apiPort = Get-FreeLoopbackPort; $webPort = Get-FreeLoopbackPort
while ($webPort -eq $apiPort) { $webPort = Get-FreeLoopbackPort }
.\scripts\start-local-acceptance.ps1 -AcceptanceRoot 'E:\Luci-local-acceptance\runs' -Port $apiPort -FrontendPort $webPort
```

Close source launch with Ctrl+C. Preserve its private `windows-baseline.json` and use the documented same-state reopen procedure. The launcher scopes/restores `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR`, `LUCI_ADDR`; never post bootstrap tokens. Configure provider endpoint/model/key or supported subscription sign-in **through Settings**, without importing unrelated host credential stores. Costs, permissions and concurrency need explicit product consent.

Build native only after fresh RAM/storage admission, from the same clean checkout in the task shell:

```powershell
Push-Location desktop
try { npm.cmd ci; if ($LASTEXITCODE -ne 0) { throw 'Locked dependency install failed' } } finally { Pop-Location }
$env:GOMAXPROCS = '2'; $env:CARGO_BUILD_JOBS = '1'
pwsh -NoProfile -File .\scripts\build-release.ps1 -Version 0.5.0 -Native
if ($LASTEXITCODE -ne 0) { throw 'Preserve the build failure and logs' }
```

This stages matching **guides/runtime/licences/cloud/sidecar** before Tauri. Require `artifacts/releases/0.5.0/release-manifest.json` to name 0dc with `sourceDirty=false`. Keep the complete built app/resource tree; do not copy only `luci.exe` or execute the NSIS installer on the personal profile.

Use [isolated native launch](LOCAL-WINDOWS-ACCEPTANCE.md#build-and-run-the-matching-native-app): a dedicated shell, fresh non-linked E: GUID directories for `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR`, `WEBVIEW2_USER_DATA_FOLDER`, with `LUCI_API` absent. Native chooses its own free port; `runtime_info.apiUrl` is authoritative, not inherited `LUCI_ADDR`. Before owner setup require exact isolated `stateDir`, empty `startupError` and owned app/sidecar identities; preserve normal state and confirm normal close/reopen stops only owned processes. No new secret/config names are introduced.

## Five priority cases, when each resource gate opens

1. **Identity and isolation:** verify native staged resources/manifest, private bootstrap, returning-owner reopen and normal-state preservation; retain exact owned cleanup. Earlier builds are not native 0dc proof.
2. **Four-project interaction/context:** distinct goals/constraints and saved purposes, clear stages, stable switching/search, keyboard/phone controls and draft restore/discard; held display GET/navigation/read revocation must not restore stale content or cancel a pending write acknowledgement.
3. **Useful coordinated work:** start with one admissible worker; review exact task context, naturally completed output and integrations across four projects. Exercise pause/resume/priority/cancel without assuming project order; a blocked/unknown/conflicted task must remain truthful and keep dependencies waiting. Record real provider output/checks/usage separately from scripted fixtures.
4. **Recovery/authority:** fresh owned controller restart and supported Stop/reconcile; preserve output/history, no replay and exact charge release. Keep original-review-before-first-observation, concurrent charged-witness isolation, terminal-before-Create refusal and fresh peer admission after restart on the open matrix. Never edit a ledger or repurpose failed fixtures.
5. **Sustained resource/quality:** follow [capacity acceptance](SWARM-CAPACITY-ACCEPTANCE.md), including all owned workers/tests/previews/browsers/containers, idle→mixed four-project load→settle, host responsiveness and fair verified throughput. Retain 300/100 ms automation targets, raw failures and observer limits; do not substitute UI-only RAM or subtract instrumentation to claim a pass.

Report stable issue ID, exact revision, reproduction and sanitized evidence as findings emerge; continue independent permitted cases while fixes proceed. Preserve all failed evidence, change source only between batches, and wait for the next immutable fix notice for CLOUD-045/046. No Windows process was launched to prepare this checkpoint.
