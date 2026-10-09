# Windows source test checkpoint — 67a5b7c7

**Retrievable test checkpoint; the local product is not release-accepted.** Use exact source [`67a5b7c75ca6354d43085f5bb81ec53ee4291f34`](https://github.com/lcfrai/luci/commit/67a5b7c75ca6354d43085f5bb81ec53ee4291f34), on `codex/luci-production-runs`. [Matching clean Linux package evidence](evidence/package-67a5b7c7/README.md) records **955 files, 347 licensed components, 46 guides and 230 revision-bound URLs**. Linux archives remain private cloud artifacts, not downloadable Windows releases; retrieve source and build Windows with its own manifest/checksums.

| Linux artifact | Bytes | SHA256 |
| --- | ---: | --- |
| Portable archive | 69,040,919 | `8d389ccc4a87d33e04dd7e3e96123bb105759b45535bd9077cccc3e3b6904157` |
| Embedded cloud archive | 34,616,061 | `180641ba23eda51b9e316186c1cad47ea42d8bf2c717b80e218a86f30a89a5f1` |

Additive clean-package verification report SHA256: `83bd266d519859e978956ee4bf5e72d8360fdab703742848499fe1fd983f013b`. Package verification is not a RAM, latency, native runtime or model-quality result.

The source/setup/configuration contract is unchanged from [99007404](TEST-CHECKPOINT-99007404.md). This checkpoint adds:

- **CLOUD-048:** after a retained failed start is explicitly retired, **Go to queue review** focuses the existing queue authorization control or its unavailable-state region. It performs no mutation and opens no consent. New work still requires reviewed-task consent and **Authorize queued tasks**; old diagnosis/history remain visible. [Evidence](evidence/cloud048-recovery-guidance-20261009/README.md).
- **CLOUD-049:** the sustained helper now opens the actual compact **Projects** drawer before **All projects**. Four exact project destinations and overflow assertions remain; workload and responsiveness targets are unchanged. This corrects the helper, not a missing app navigation route. [Evidence](evidence/cloud049-phone-overview-20261009/README.md).
- **CLOUD-050:** initially closed definition groups defer their editors until first expansion, retain them after collapse, and preserve controlled document values, question focus, draft recovery and missing-link repair. Four large controlled definitions reduced initial per-project DOM elements from 6,124 to 867; that is not measured installed latency or RAM improvement. [Evidence and exact large fixtures](evidence/cloud050-deferred-definition-editors-20261009/README.md).
- **CLOUD-051:** closing a compact unsaved-draft navigation dialog restores a usable current control instead of the now-inert rail opener; ordinary desktop focus remains unchanged. [Evidence](evidence/cloud051-draft-navigation-focus-20261009/README.md).

Validation boundaries: **2,787 assertions / 158 files passed before the two C051 files changed**. Final C051 affected coverage is **45/45**; final combined **TypeScript exits 0**, with 1,185 repository and 19,369 dependency paths unchanged during that check. Counts overlap; this is not a fresh full-suite pass for the final C051 bytes. [Compiler/source relationship](evidence/cloud051-final-typecheck-20261009/README.md). Styled component/browser checks are controlled Linux source evidence, not Windows or installed-provider acceptance.

The prior [sustained 99007404 attempt 7](evidence/sustained-99007404-attempt7/README.md) records **five natural scripted-provider task completions and five reviewed integrations across four projects, including the amendment**, plus a running-command controller restart. Overall it remains **FAILED**: the later phone assertion failed before `functionalPassed`, and idle/loaded/settled switch p95 **516.83 / 644.91 / 569.89 ms** exceeded the unchanged 300 ms target. Input p95 **88.91 / 82.25 / 78.43 ms** met 100 ms. No live external model was exercised. The new 67a5b7c7 product has **no matching installed responsiveness or sustained-workflow result yet**; neither correction retroactively passes the failed run.

## Current Windows hold

The latest relayed Windows available-RAM reading is **36,548,608 bytes**. Source/app/browser/native-build workloads remain held; this notice does not request their launch. Recheck fresh available physical RAM and commit before each later batch; retain the established 2 GiB source and 2.5 GiB browser/native launch gates and product admission checks. Do not stop unrelated applications or weaken CPU/memory/storage guards to fit a test.

The originating local testing thread owns Windows execution. Preserve the original C: checkout and normal Luci state, all isolated E: checkouts/evidence, relocated Docker data and the original 107-container/four-running identity baseline. No process on Dylan's machine was started to prepare this checkpoint. Never run Linux exclusive/empty-daemon drivers on shared Docker Desktop or bypass `test-installed-windows.ps1`'s disposable-runner restriction.

## Exact retrieval and unchanged setup, when resources permit

Reuse the established task-local **Node 24.19.0/npm 10.9.3, Go 1.26.3**, E: TEMP/TMP/caches and activation. Native packaging additionally uses **PowerShell 7, Python, Rust/cargo 1.99 MSVC, C++ Build Tools/Windows SDK and WebView2**. Preserve global PATH, SDK configuration, existing installations and credential stores. Measure actual Docker guest storage and its physical E: backing volume, not guest free space alone.

Move revisions only between closed test batches. In the existing task-local PowerShell shell, create a fresh isolated checkout:

```powershell
$ErrorActionPreference = 'Stop'
$candidate = '67a5b7c75ca6354d43085f5bb81ec53ee4291f34'
$checkout = Join-Path 'E:\Luci-local-acceptance\checkouts' ('67a5b7c7-' + [guid]::NewGuid().ToString('N'))
git clone --no-checkout https://github.com/lcfrai/luci.git $checkout
if ($LASTEXITCODE -ne 0) { throw 'Clone failed' }
git -C $checkout fetch origin codex/luci-production-runs
if ($LASTEXITCODE -ne 0) { throw 'Fetch failed' }
git -C $checkout checkout --detach $candidate
if ($LASTEXITCODE -ne 0) { throw 'Pinned source unavailable' }
Set-Location $checkout
if ((git rev-parse HEAD).Trim() -cne $candidate -or @(git status --porcelain).Count) { throw 'Require exact clean source' }
Push-Location desktop
try { npm.cmd ci; if ($LASTEXITCODE -ne 0) { throw 'Locked dependency install failed' } } finally { Pop-Location }
```

Record exact HEAD and source hashes before/after; keep normal remote-tracking provenance. No synthesized refs, global runtime changes or reuse of failed fixtures.

For source UI testing, choose distinct free loopback ports. The existing launcher rechecks them and creates private fresh state/projects/evidence on E:; launching it does not request Docker runtime installation.

```powershell
function Get-FreeLoopbackPort {
  $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
  try { $listener.Start(); return $listener.LocalEndpoint.Port } finally { $listener.Stop() }
}
$apiPort = Get-FreeLoopbackPort; $webPort = Get-FreeLoopbackPort
while ($webPort -eq $apiPort) { $webPort = Get-FreeLoopbackPort }
.\scripts\start-local-acceptance.ps1 -AcceptanceRoot 'E:\Luci-local-acceptance\runs' -Port $apiPort -FrontendPort $webPort
```

Close with Ctrl+C, retain its private `windows-baseline.json`, and use the documented same-state reopen procedure. The launcher scopes/restores `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR` and `LUCI_ADDR`. Never publish bootstrap tokens. Configure a permitted model endpoint/model/key or supported subscription sign-in through Settings; do not import unrelated host credential stores. Settings runtime-install actions and worker execution remain separately gated and explicitly authorized through the product.

For a matching native build after fresh resource admission:

```powershell
$env:GOMAXPROCS = '2'; $env:CARGO_BUILD_JOBS = '1'
pwsh -NoProfile -File .\scripts\build-release.ps1 -Version 0.5.0 -Native
if ($LASTEXITCODE -ne 0) { throw 'Preserve the build failure and logs' }
```

Require `artifacts/releases/0.5.0/release-manifest.json` to name the exact candidate with `sourceDirty=false`. This route stages matching guides/runtime/licences/cloud/sidecar resources before Tauri. Keep the complete app/resource tree; do not copy only `luci.exe` or execute NSIS on the personal profile.

Follow [isolated native launch](LOCAL-WINDOWS-ACCEPTANCE.md#build-and-run-the-matching-native-app): dedicated shell; fresh non-linked E: GUID directories for `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR` and `WEBVIEW2_USER_DATA_FOLDER`; `LUCI_API` absent. Native chooses its own free API port; `runtime_info.apiUrl` is authoritative. Verify exact isolated `stateDir`, empty `startupError` and owned app/sidecar identities before owner setup. No new configuration/secret names are introduced.

## Prioritized acceptance batches after the hold clears

1. **Startup/isolation:** verify exact package/resources, isolated bootstrap, returning owner, same-state reopen and normal-state preservation. Ordinary close must stop only this app's sidecar/WebViews; preserve Docker's unrelated inventory. Carry older native passes at their original revisions, not as fresh 67a5 evidence.
2. **Four projects and responsive actions:** create alpha/beta/gamma/delta with unique goals, constraints and purposes; switch/search without context confusion while Home reads are delayed. Exercise 1440→390→1440, Projects open/Close/Escape, All projects and each destination. With an unsaved draft, Keep editing/Escape must retain its project/value and restore a visible usable control; Keep drafts and continue, another project, reopen and Restore draft must recover only the original project's edits.
3. **Large shared context:** use the published four-project large fixtures within the unchanged 4 MiB total draft limit. Initially closed group editors should stay unmounted until opened; summary counts and warnings remain visible. Test Space/Enter, collapse/reopen preserving the same edited value, exact question focus after recovery, missing requirement-link repair and Read/Edit revocation/restoration. Check real owned memory and interaction behavior separately; lower DOM counts do not establish a RAM/latency pass.
4. **Retained failure and useful work:** with a positively recorded pre-Create refusal, inspect its exact diagnosis, then explicitly **Retain changes and retry → Retain output and allow retry**. **Go to queue review** must focus the eligible **Queue reviewed tasks** control, or the **Background queue review** region when unavailable; no job or consent opens automatically. Only current reviewed consent and **Authorize queued tasks** may submit a new attempt. Keep the old output/context/history and exact charges truthful. Across four projects, exercise current priority/pause/resume/cancel, accepted task output, reviewed integration and dependency blocking without assuming project order.
5. **Recovery, quality and sustained resources:** separately test owned controller restart/Stop/reconciliation, exact settlement and charge release, no replay, conflict review and real-provider output/usage. Retain open review-before-first-observer, concurrent charged-witness isolation, terminal-before-Create refusal and fresh peer admission after restart cases. Follow [capacity acceptance](SWARM-CAPACITY-ACCEPTANCE.md) for idle→mixed four-project load→settle, all owned processes/containers and whole-computer responsiveness. Preserve 300/100 ms automation targets, raw failed rows and observer limits; no UI-only memory accounting or subtraction to declare a pass.

Windows packaging/runtime, installed corrected recovery, live-provider quality and complete four-project performance remain open. Report stable issue IDs, pinned revision, reproduction and sanitized screenshots/logs/resource evidence as findings emerge; the implementation thread fixes in parallel while the local thread continues independent permitted tests. Switch revisions only between completed batches. [Two-way protocol and prior scoped gates](LOCAL-TEST-LOOP.md) remain authoritative alongside the [full Windows procedure](LOCAL-WINDOWS-ACCEPTANCE.md).
