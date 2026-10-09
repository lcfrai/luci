# Windows test checkpoint — 78665270

Exact source: [`786652704fa8bebd1ba1af41cfa1d414d7def100`](https://github.com/lcfrai/luci/commit/786652704fa8bebd1ba1af41cfa1d414d7def100), branch `codex/luci-production-runs`. **Not LOCAL TEST HANDOFF READY or release acceptance.** [Linux package receipts](evidence/package-78665270/README.md) bind the clean build and 964 matching members; archives remain publisher-held. Windows/native acceptance is unverified.

[Cold attempt2](evidence/cloud073-installed-78665270-cold-attempt2/README.md) passed the scoped Linux scenario, driver and wrapper: exits 0/7, one Restart and both natural charge releases. [Cold attempt1](evidence/cloud073-installed-78665270-cold-attempt1/README.md) remains failed overall. [Warm attempt2](evidence/cloud073-installed-78665270-warm-attempt2/README.md) failed with one pending Restart and incomplete cleanup; held-GET fulfillment does not establish Restart acknowledgement or complete warm ordering. Known-owned closure and unresolved reservations retain their literal per-run scopes.

[CLOUD-075 installed authority witness](evidence/cloud075-installed-78665270-20261009/README.md) passed: six deliberately delayed entry/reload windows each had one permission GET; a separate held real grant did not restore denied private context. This is request/race evidence, not latency. Controlled tests: old ten cases 2 pass/8 fail → fixed 10 pass; initial existing 64 pass/2 spy failures plus unused rejection, corrected Provider 28/28 plus 38 previously passed cases; TypeScript passed. These are 76 distinct final cases across runs, not one clean suite. Independent closure found five known identities absent and two browser zombies.

Runtime `sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8` was built at 5f4a966f; 106 captured inputs match 786. C070's 61 focused passes, 38 related passes/1 helper skip and compile-only bucket ran on 29931afc plus overlay; 944 inputs match 786. Source parity is not new execution. [Earlier b94 scopes](TEST-CHECKPOINT-B94AD37E.md) remain historical.

## Retrieve and launch an isolated source run

Use a dedicated shell/E: paths. Keep the 40 GiB floor; check Docker Linux/backing disk, RAM and commit. Preserve shared work. Use task-local Node 24.19.0/Go 1.26.3; record npm. Docker Desktop needs Linux. No new settings/secrets.

```powershell
$ErrorActionPreference = 'Stop'
$candidate = '786652704fa8bebd1ba1af41cfa1d414d7def100'
if ((node --version).Trim() -cne 'v24.19.0') { throw 'Activate Node 24.19.0' }
if ((go version) -notmatch '^go version go1\.26\.3 ') { throw 'Activate Go 1.26.3' }
npm.cmd --version
$checkout = Join-Path 'E:\Luci-local-acceptance\checkouts' ('78665270-' + [guid]::NewGuid().ToString('N'))
git clone --no-checkout https://github.com/lcfrai/luci.git $checkout
if ($LASTEXITCODE) { throw 'Clone failed' }
git -C $checkout fetch origin codex/luci-production-runs
if ($LASTEXITCODE) { throw 'Fetch failed' }
git -C $checkout checkout --detach $candidate
if ($LASTEXITCODE) { throw 'Pinned checkout failed' }
Set-Location $checkout
if ((git rev-parse HEAD).Trim() -cne $candidate -or @(git status --porcelain).Count) { throw 'Require exact clean source' }
Push-Location desktop
try { npm.cmd ci; if ($LASTEXITCODE) { throw 'Locked install failed' } }
finally { Pop-Location }
```

Choose distinct free loopback ports using the [isolated procedure](LOCAL-WINDOWS-ACCEPTANCE.md#retrieve-the-exact-candidate-and-start-safely), then:

```powershell
.\scripts\start-local-acceptance.ps1 -AcceptanceRoot 'E:\Luci-local-acceptance\runs' -Port $apiPort -FrontendPort $webPort
```

The launcher checks ports, creates fresh state and temporarily clears inherited `LUCI_*`. `-PrepareOnly` does not launch. **Continue to projects** needs no provider; shells need runtime, authority and admission. Keep bootstrap/state private; Ctrl+C closes the run. Follow documented same-state reopen steps.

## Native route and remaining checks

After resource admission and Rust/MSVC/SDK/WebView2 prerequisites:

```powershell
pwsh -NoProfile -File .\scripts\build-release.ps1 -Version 0.5.0 -Native
if ($LASTEXITCODE) { throw 'Retain failed native build output' }
```

Require exact clean-source manifest and matching app/sidecar/resources. Follow [isolated native launch](LOCAL-WINDOWS-ACCEPTANCE.md#build-and-run-the-matching-native-app): fresh non-linked state/project/WebView paths; `LUCI_API` absent in own-sidecar mode; `runtime_info` authoritative. Installer generation is not execution. Preserve personal state/failed fixtures.

Test authority/draft isolation, keyboard flows, manual retirement, explicit warm/cold Restart, recovery, four-project work and real providers. Preserve uncertainty without replay. Targets remain 300 ms switch/100 ms input; 29931afc switch p95 494.921816 ms failed. Windows/native, latency, model quality and idea-to-live completion remain open. Setup draft 43 is saved/readback-verified at 786, unpublished; no fresh-task restoration. Its discovery predates subsequent cold2, C075 and warm fixtures; repository declaration unchanged.
