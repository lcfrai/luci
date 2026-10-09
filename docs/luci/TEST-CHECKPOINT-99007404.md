# Windows source test checkpoint — 99007404

**Useful test checkpoint; local release acceptance is still open.** Source is publicly retrievable at [`99007404a5ddd8592dee376ea00d99c6180d26fe`](https://github.com/lcfrai/luci/commit/99007404a5ddd8592dee376ea00d99c6180d26fe), on `codex/luci-production-runs`. [Package receipts and reproduction helpers](evidence/package-99007404/README.md) bind this exact clean build. Linux archives remain private cloud artifacts, **not downloadable releases**; use the exact source to build Windows.

Clean Linux packaging matched **954 files, 347 licensed components, 45 guides and 217 revision-bound hosted URLs**. Portable archive: **69,027,325 bytes**, SHA256 `9bf32d8ebe0c7c7d3297c81e91f0451f0035357cb77eb7abf06099920dd9f164`. Embedded cloud archive: **34,608,456 bytes**, SHA256 `620246cf1f48a7a9b4f8df27a026dbd042d7cd84e4501b9afefa46e63e276e05`. These identify Linux artifacts; a Windows build has its own manifest/checksums. Archive reduction is not a RAM or latency result.

CLOUD-045's helper reports exact terminal queue refusals promptly. CLOUD-046 persists the actual final CPU/memory admission operands, available only through the host-authorized single-session GET; normal public reasons, team/member privacy, admission and retry behavior are unchanged. CLOUD-047 puts stable shadcn project destinations before asynchronous Home content. All **2,778 frontend assertions / 158 files plus TypeScript pass**; affected backend checks pass separately. See the linked [worklist](QUALITY-WORKLIST.md) for exact original/final sources and scoped counts.

A [matching fresh installed idle diagnostic](evidence/cloud032-idle-99007404-20261009/README.md) completes 100 switches, 200 inputs and 100 resets, with no workers/models. **Switch p95 561.36 ms still fails 300 ms; input p95 81.12 ms passes 100 ms.** Its diagnostic completion is not responsiveness acceptance. The full matching four-project workload is pending. Older sustained attempt 6 at exact `0dcafb14` remains failed: A/C/D naturally exit, A/C integrate and B is refused before allocation. The original CPU reading remains unavailable; new diagnostics cannot reconstruct it.

Native 990 packaging/runtime, real-provider quality, remaining recovery cases and sustained four-project performance remain unverified. Prior Windows passes retain their original revision/scope. This checkpoint does not request a workload while the local RAM gate is closed.

## Preserve the machine; prepare only when resources permit

Keep the original C: checkout, normal app data, existing E: evidence and all unrelated applications/workloads. Use the tester's task-local **Node 24.19.0/npm 10.9.3, Go 1.26.3**, E: caches/TEMP/TMP; native packaging also needs **PowerShell 7, Python, Rust/cargo 1.99 MSVC, C++ Build Tools/Windows SDK and WebView2**. Reuse established activation; do not modify global PATH or SDK settings. Measure fresh available RAM/commit and actual Docker guest/backing-volume space before each batch; do not relax admission or stop ordinary apps to fit it.

Docker's relocated E: disk and existing 107-container/four-running baseline must be preserved by exact identities. **Do not run Linux exclusive/empty-daemon drivers on this shared Docker Desktop or bypass `test-installed-windows.ps1`'s disposable-runner guard.** See [full Windows procedure](LOCAL-WINDOWS-ACCEPTANCE.md) and [two-way test loop](LOCAL-TEST-LOOP.md); this checkpoint supplies the newer exact source identity.

## Retrieve a fresh isolated checkout

The local tester already has a verified isolated **0dcafb14** prepare-only baseline. Preserve it and prior evidence; move to 990 only between closed test batches, when resources permit. The startup/configuration contract is unchanged, so there is no need to repeat unchanged preparation solely for this document. For a fresh candidate checkout in the task-local PowerShell shell:

```powershell
$ErrorActionPreference = 'Stop'
$candidate = '99007404a5ddd8592dee376ea00d99c6180d26fe'
$checkout = Join-Path 'E:\Luci-local-acceptance\checkouts' ('99007404-' + [guid]::NewGuid().ToString('N'))
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

This stages matching **guides/runtime/licences/cloud/sidecar** before Tauri. Require `artifacts/releases/0.5.0/release-manifest.json` to name 990 with `sourceDirty=false`. Keep the complete built app/resource tree; do not copy only `luci.exe` or execute the NSIS installer on the personal profile.

Use [isolated native launch](LOCAL-WINDOWS-ACCEPTANCE.md#build-and-run-the-matching-native-app): a dedicated shell, fresh non-linked E: GUID directories for `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR`, `WEBVIEW2_USER_DATA_FOLDER`, with `LUCI_API` absent. Native chooses its own free port; `runtime_info.apiUrl` is authoritative, not inherited `LUCI_ADDR`. Before owner setup require exact isolated `stateDir`, empty `startupError` and owned app/sidecar identities; preserve normal state and confirm normal close/reopen stops only owned processes. No new secret/config names are introduced.

## Five priority cases, when each resource gate opens

1. **Identity and isolation:** verify native staged resources/manifest, private bootstrap, returning-owner reopen and normal-state preservation; retain exact owned cleanup. Earlier builds are not native 990 proof.
2. **Four-project interaction/context:** distinct goals/constraints and saved purposes, clear stages, stable switching/search (including delayed Home capacity/summary replies), keyboard/phone controls and draft restore/discard; held display GET/navigation/read revocation must not restore stale content or cancel a pending write acknowledgement.
3. **Useful coordinated work:** start with one admissible worker; review exact task context, naturally completed output and integrations across four projects. Exercise pause/resume/priority/cancel without assuming project order; a blocked/unknown/conflicted task must remain truthful and keep dependencies waiting. Record real provider output/checks/usage separately from scripted fixtures.
4. **Recovery/authority:** fresh owned controller restart and supported Stop/reconcile; preserve output/history, no replay and exact charge release. Keep original-review-before-first-observation, concurrent charged-witness isolation, terminal-before-Create refusal and fresh peer admission after restart on the open matrix. Never edit a ledger or repurpose failed fixtures.
5. **Sustained resource/quality:** follow [capacity acceptance](SWARM-CAPACITY-ACCEPTANCE.md), including all owned workers/tests/previews/browsers/containers, idle→mixed four-project load→settle, host responsiveness and fair verified throughput. Retain 300/100 ms automation targets, raw failures and observer limits; do not substitute UI-only RAM or subtract instrumentation to claim a pass.

Report stable issue ID, exact revision, reproduction and sanitized evidence as findings emerge; continue independent permitted cases while fixes proceed. Preserve all failed evidence, change source only between batches, and return CLOUD-045/046/047 results at their exact tested scope; these fixes still require installed/native verification. No Windows process was launched to prepare this checkpoint.
