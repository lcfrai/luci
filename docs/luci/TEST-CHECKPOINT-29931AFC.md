# Windows test checkpoint — 29931afc

Exact source: [`29931afc5b31ae9b9bd7b8973a7e3fdb840609a3`](https://github.com/lcfrai/luci/commit/29931afc5b31ae9b9bd7b8973a7e3fdb840609a3), branch `codex/luci-production-runs`. Source and evidence are retrievable. **This is a focused test checkpoint, not LOCAL TEST HANDOFF READY or release acceptance.** The matching Linux package is verified. Installed terminal attempts 9–11 each verify first completion and Restart readiness but fail before forwarding Restart. Attempt 9 releases its charge through supported Stop; attempts 10 and 11 retain their own exact charged leases despite container absence. Native Windows, complete Restart and live-model acceptance remain open.

There are no new configuration or secret names. CLOUD-063 makes a typed terminal exit promptly update the exact allocation's session status without waiting for the periodic sweep; it retains output and the charged reservation. CLOUD-065 lets an initially cleaned exited session attach after its current successful Restart acknowledgement. Neither a process exit nor a successful Restart means work is verified or capacity released.

| Artifact identity | Current status |
| --- | --- |
| Source | `29931afc5b31ae9b9bd7b8973a7e3fdb840609a3` |
| Linux release manifest SHA256 | `fa68f997a3f1db984b1fd6e6284e3e0d36c11e7ee9148eb977c99cfb095e7b77`; exact source, `sourceDirty=false` |
| Linux portable archive | 69,117,092 bytes; SHA256 `0b6eb1719c1ce4051eeb1cec55eef8625961f79cd2ffc1e14fb953a568fcefa0` |
| Linux cloud archive | 34,651,655 bytes; SHA256 `85d91fd12ece7d6edbaa4ec5d364526ee0c73c4af731202c366a4a1b38f2c5da` |
| Reused runtime image and binding | `sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8`; built at `5f4a966f`, all 106 captured runtime inputs match exact `29931afc` |
| Windows package/native execution | Not yet verified at this revision |

The original Linux verifier reports 962 matching extracted members, 347 licensed components and zero missing licence entries. API binary SHA256 is `d35e990d90f663772aa951bf19150e996680974cd5c627b8c8b7ce0da26314dd`; web index SHA256 is `f99796b77f74be2fed35d0fa5b0dbf6729b6552905831e402f9a6515a231825b`. [Package receipts](evidence/package-29931afc/README.md) bind the publisher-held archives; no downloadable binary or fresh independent download is asserted. Runtime reuse is source-input parity, not a rebuild at 29931afc. The separate retained-runtime archive and current-daemon restoration are now recorded below.

A Linux package receipt does not supply a Windows binary or prove Windows execution. Build the Windows candidate from this exact source and retain its own manifest/checksums.

## Cloud setup state, separate from Windows acceptance

Required Go 1.26.3 restoration completed: the canonical cloud tool tree is again a real `/workspace/.luci-tools/go1.26.3` directory, with its former TMP location retained as a compatibility alias. The [restoration receipts](evidence/go-tool-restoration-20261009/README.md) preserve original failures, controls and completed operation journals; this is storage preparation, not Windows execution or a fresh snapshot test.

Runtime 760's retained gzip is 1,013,403,794 bytes, SHA256 `637d1d02c701be5f4327bb76dcb58335e1665b27945e3f1cb08bb825c5fd96fd`. Verification includes the image/config/layer identities, complete gzip CRC and exact sandboxd/entrypoint bytes and modes. A separate actual installer check passed after the development tag was absent: it loaded that archive into the **same existing daemon**, which still held the same image/layers under another tag. This proves that missing-tag path, not loading into an empty daemon or restoring another task. [Archive, restoration and setup receipts](evidence/setup-runtime760-29931afc-20261009/README.md) preserve those distinct scopes and the older pre-check `dockerLoadVerified=false` receipt unchanged.

The complete install/start fields were saved with `status=saved` and `requires_publish=true`, preserving the selected 29931afc repository and existing network/secret/environment settings. **The draft was not published; saved fields were not thereby executed and no new snapshot restoration is verified.** Optional historical artifacts remain separately retained; storage placement does not replace fresh CPU/memory/disk/authority admission. The Windows workflow below retains its own toolchain, state and runtime checks.

## Retrieve and start without touching existing work

The originating local testing thread owns Windows execution. Recheck physical RAM and commit before every batch; retain its established minimum launch gates of 2 GiB for source work and 2.5 GiB for browser/native work, alongside product admission checks. These are launch gates, not four-project performance budgets. Do not stop unrelated applications, prune Docker, reuse installed state or relax CPU/memory/storage guards. Preserve existing checkouts, evidence and the recorded shared Docker inventory.

Use the established task-local Node 24.19.0/npm and Go 1.26.3 activation, with TEMP/TMP/caches on roomy E:. The tester's npm 10.9.3 and cloud npm 11.9.0 are distinct recorded toolchains. Do not change global PATH or import host credential stores. Move revisions only between closed batches:

```powershell
$ErrorActionPreference = 'Stop'
$candidate = '29931afc5b31ae9b9bd7b8973a7e3fdb840609a3'
$checkout = Join-Path 'E:\Luci-local-acceptance\checkouts' ('29931afc-' + [guid]::NewGuid().ToString('N'))
git clone --no-checkout https://github.com/lcfrai/luci.git $checkout
if ($LASTEXITCODE -ne 0) { throw 'Clone failed' }
git -C $checkout fetch origin codex/luci-production-runs
if ($LASTEXITCODE -ne 0) { throw 'Fetch failed' }
git -C $checkout checkout --detach $candidate
if ($LASTEXITCODE -ne 0) { throw 'Pinned checkout failed' }
Set-Location $checkout
if ((git rev-parse HEAD).Trim() -cne $candidate -or @(git status --porcelain).Count) {
    throw 'Require exact clean source'
}
Push-Location desktop
try {
    npm.cmd ci
    if ($LASTEXITCODE -ne 0) { throw 'Locked dependency installation failed' }
} finally { Pop-Location }
```

Choose separate free loopback ports. The safe launcher rechecks availability, checks Docker Desktop's Linux engine, and creates fresh private state/projects/evidence outside the checkout:

```powershell
function Get-FreeLoopbackPort {
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    try { $listener.Start(); return $listener.LocalEndpoint.Port }
    finally { $listener.Stop() }
}
$apiPort = Get-FreeLoopbackPort
$webPort = Get-FreeLoopbackPort
while ($webPort -eq $apiPort) { $webPort = Get-FreeLoopbackPort }
.\scripts\start-local-acceptance.ps1 -AcceptanceRoot 'E:\Luci-local-acceptance\runs' -Port $apiPort -FrontendPort $webPort
```

`-PrepareOnly` performs preparation without launching the app; rerunning without it creates another fresh run. The launcher scopes/restores `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR` and `LUCI_ADDR`, and clears inherited `LUCI_*` settings during launch. Do not assume an inherited `LUCI_SANDBOX_IMAGE` override survives it. Close with Ctrl+C and use the [documented same-state reopen procedure](LOCAL-WINDOWS-ACCEPTANCE.md#retrieve-the-exact-candidate-and-start-safely) for persistence tests. Retain the private baseline; never publish bootstrap tokens.

For UI-only checks, choose **Continue to projects** rather than a setup route that starts runtime installation. Shell tests need the reviewed matching runtime, current execution permission and fresh resource/storage admission; they require no model credentials. Measure both Docker's Linux disk and its physical Windows backing volume. Use the existing supported isolated-runtime procedure and record the configured image plus immutable ID; never run exclusive Linux acceptance drivers against shared Windows Docker.

For native packaging, after fresh resource admission use `pwsh -NoProfile -File .\scripts\build-release.ps1 -Version 0.5.0 -Native` with the established Rust/MSVC/SDK/WebView2 prerequisites. This stages guides and matching sidecar/resources. Require the release manifest's exact candidate and `sourceDirty=false`; keep the full app/resource tree. Follow [isolated native launch](LOCAL-WINDOWS-ACCEPTANCE.md#build-and-run-the-matching-native-app): dedicated shell, fresh nonlinked E: `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR`, `WEBVIEW2_USER_DATA_FOLDER`, and no `LUCI_API`. Own-sidecar mode chooses its free API address; `runtime_info.apiUrl` and exact `stateDir` are authoritative, not inherited `LUCI_ADDR`. Do not install NSIS over personal data or bypass `test-installed-windows.ps1`'s disposable-runner guard.

## Focused acceptance batches

Record source/package/image, instance, project, session and allocation-generation identities. Keep original failures and raw timings; do not replay a mutation to hide an uncertain result.

1. **CLOUD-063 — typed exit and current status.** In one owned disposable shell session, print a unique marker and exit 0. Retain final output, typed Exit and browser close observations separately. The session should become exited and expose permitted **Restart** without relying on the next periodic sweep; there must be no automatic reconnect. For instrumented tests, delay one old running-list response and release it after exit: it must not resurrect the old allocation. Do not count an absent close frame as a clean close. Output, allocation references and reservation stay until their separately supported cleanup proves settlement. Repeat a nonzero exit; it stays ended and is not labelled successful work.
2. **CLOUD-065 — cold cleaned-session Restart.** After supported cleanup, normally close/reopen the app so the exited session is initially loaded without its old allocation fingerprint. Its transcript remains available. Test the pane **Restart** and, in a separate case, menu **Restart clean**. A single explicit request must restart the same session ID; attach begins only after a successful current acknowledgement carrying the new generation. Do not create a second session. Verify new output reaches that terminal exactly once, then test another exit. Hold/reject an acknowledgement where the tester has a controlled transport: a late running poll alone, an uncertain request, permission revocation or a changed workspace must not open a socket/replay Restart. Preserve the failed/unknown outcome for explicit reconciliation.
3. **CLOUD-056/CLOUD-060 — actual workspace portals.** Using disposable team owner/member accounts, retain a definition draft, then remove membership while **Team access** is open at 1440px and **Team settings** at 390px. A fresh typed team-access refusal must hide private presentation and release modal pointer/scroll effects; the next Tab must reach visible controls outside the hidden workspace. Retry after a transient membership-read failure must retain the draft. Repeated denials must not initiate a membership request for every poll. Restore membership without the project grant: old private project content must stay hidden. Regrant, reopen, restore the correct draft and save explicitly. Test private project-specific review/recovery dialogs separately; the Linux run below did not exercise every dialog. Keep stale account/project/allocation responses and native focus behavior in the matrix.
4. **Four-project and resource follow-through.** Continue alpha/beta/gamma/delta switching, drafts, context isolation, queue controls, reviewed integration, recovery and actual provider work from the [full matrix](LOCAL-WINDOWS-ACCEPTANCE.md#integrated-manual-acceptance-matrix) and [capacity procedure](SWARM-CAPACITY-ACCEPTANCE.md). Include all owned child/container costs and ordinary-computer responsiveness. Retain the unchanged 300 ms switch / 100 ms input automation targets; functional success and low UI-only RAM do not pass them.

Optional focused source checks, when admitted, can precede runtime: `go test -C control-plane -p 1 -count=1 ./internal/api -run '^TestAttach(TypedExit|LateTypedExit|Terminal)'`; in `desktop`, `npm exec vitest -- run src/api/attachRegistry.exit.test.ts src/components/develop/SessionPane.restartAttach.test.tsx --maxWorkers=1 --no-file-parallelism`, then `npm run typecheck`. These controlled checks do not establish Docker/native execution; do not repeat unchanged completed coverage unnecessarily.

## Evidence carried forward and what remains open

The integrated candidate passed 140 selected frontend assertions across four files, TypeScript, and nine Go race tests / 30 outcomes. These overlap the earlier affected runs and are not a full-suite or installed pass. [Combined source evidence](evidence/cloud063065-integrated-source-20261009/README.md), [C063](evidence/cloud063-session-terminal-liveness-20261009/README.md), [C065](evidence/cloud065-cold-session-restart-20261009/RECEIPT.md).

**29931afc terminal attempt 9 remains failed overall, with installed first-completion positives.** The real shell delivered FIRST, one typed `exit status 0`, then browser Close 1000 with `wasClean=true` and no socket error. The ended footer, no reconnect overlay, suppressed stdin and no extra socket passed while the old running-list response was held; fresh exited status and Restart readiness subsequently passed. The unchanged helper then refused its six-GiB fixture-root floor before forwarding: both API and browser-forwarded Restart counts are zero. No replacement attachment, second exit 7, warm Restart or cold Restart result is credited. Refusal-time free bytes were not recorded. Supported Stop released this fixture's charge; separate exact-container removal is not automatic cleanup. Independent closure retains two exact PID-1 Chromium zombies. [Original attempt 9 and scoped closure](evidence/cloud054-installed-29931afc-attempt9/README.md).

**CLOUD-067/CLOUD-068 helper sequencing is controlled-verified; installed follow-through remains pending after attempt 10.** C067 v9 waits up to 35 seconds for exact natural product settlement before opening the second stale-read gate, preserving the six-GiB floor and making no Stop/Destroy/release mutation. Its final same-source controls retain 13 failures/one pass on v8 and 14 passes on v9. C068 v10 retains the eight-second old-read gate, ten-second product read budget and 150-second Restart budget: a slow Create releases the authentic read before its deadline, without replay or substitution. A completed warm lifecycle with the required old-response-after-acknowledgement ordering unexercised must still report overall `passed:false`. Its four final controls retain three failures/one pass on v9 and four passes on v10; controlled 36/145-second cases are not real measured workloads. These are external helper corrections, not changed product admission or installed successes. Attempt 10 failed in the preceding settlement observer, so C068's actual Restart/read-ordering paths were not reached. The old-before-Restart response ordering, full warm lifecycle and separate C065 cold-registry scenario remain distinct gates.

**29931afc terminal attempt 10 remains failed, with its original charged lease preserved (CLOUD-069, P2).** First real exit 0, clean browser Close 1000, exited status and Restart readiness passed again; zero Restart POSTs were forwarded. The installed helper stopped at “Allocation state unconfirmed” before recording the Docker status it rejected. That status remains unknown; no particular cleanup state or product defect is inferred retrospectively. Independent closure records the container absent while one exact manual lease remains charged and unchanged, plus two PID-1 Chromium zombies. The original fixture was not repaired, and neither container absence nor helper closure proves lease release or recovery. This differs from attempt 9's separately evidenced supported Stop release. Closure SHA256: `c33f3914afec08dccbf3bdf82e854bbf099a9102f413b1d55d2c9b6df5636883`.

C069's v11 follow-up records the recognized cleanup transitions described below, keeping the earlier rejected status unknown. It preserves the deadline, identity/charge guards and requirement for exact absence plus charge release. It does not bypass admission, repair the ledger or credit attempt 10 with a later result. C068's controlled results are unchanged; its installed ordering and the full warm/cold Restart gates remain pending.

**29931afc terminal attempt 11 also fails before Restart, with a known settlement sequence.** CLOUD-069 v11 warm again reaches first exit 0, clean Close 1000, ended/no-reconnect/no-extra-socket assertions and fresh Restart readiness. Its 35-second natural-settlement loop records `exited` → `removing` → absent while one exact 1,024 MiB / 1,000 mCPU manual lease stays charged. The actual final helper error is `spawnSync docker ETIMEDOUT`, not attempt 10's unknown-state assertion. Both Restart counts remain zero; no replacement attachment, second exit 7 or warm/cold Restart outcome was reached. The phase labelled “supported-session-stop” contains no recorded session DELETE; its label does not establish Stop or release. Independent closure retains the exact lease and two PID-1 Chromium zombies, with selected containers/volumes absent. [Original attempt 11 and closure](evidence/cloud054-installed-29931afc-attempt11/README.md).

The existing 29931afc product can reconcile a confirmed-absent, settled, exited manual lease during a later eligible worker admission. No such later admission/release was performed for this retained fixture. These observations do **not** prove a permanent leak. **CLOUD-070 is a later integrated correction outside this pinned checkpoint.** Its exact original regression fails; the fixed source passes 61 focused and 38 related race-enabled outcomes, with one intentional standalone child-helper skip and its real cross-process parent passing. All 27 control-plane packages compile; two additional packages have no tests. [Source evidence](evidence/cloud070-manual-lease-observation-20261009/README.md) does not turn the 29931afc binary into a fixed package. Use a separately identified matching revision/package for C070 installed testing. Failed fixtures and their charges remain untouched.

Prior **5f4a966f terminal attempt 8 remains failed**. Its first typed exit 0 and browser Close 1000 were observed, but the unchanged fresh-status/Restart wait failed. No restart or second exit is credited. Supported Stop released that fixture's charge; separate exact helper removal and two retained PID-1 browser zombies are not automatic cleanup. Attempt 9 above advances only the explicitly reached first-completion and fresh-status stages; complete Restart and native verification remain open. [Original attempt and closure](evidence/cloud054-installed-5f4a966f-attempt8/README.md).

Prior **5f4a966f portable API/web+Chromium C056/C060 workflow passed its functional assertions**, covering only Team access 1440px, Team settings 390px and the definition draft/regrant/save path. Each blocked entry view reported zero axe violations/eight passes/zero incomplete. The session catalogue was empty: no actual cached terminal-output hiding, model or agent execution was tested. Driver/scenario passed, but the supervisor remained false because two exact crashpad zombies and incomplete process observations prevented full cleanup certification. This is evidence at its original product/helper pin, not a new 29931afc native or all-dialog pass. [Scoped browser evidence](evidence/cloud060-installed-5f4a966f-20261009/README.md).

Matching Linux package verification and runtime-input parity are established at the scopes above. Complete installed terminal Restart/nonzero-exit and old-response ordering, C065 cold restart, native Windows, live-provider quality, complete four-project resource/responsiveness and retained recovery gates remain open. The latest actual29931afc diagnostic completes all 432 original actions (100 switches, 200 inputs, 100 resets and 32 separate trace follow-ups). Primary switch p95 **494.921816 ms fails 300 ms**; input p95 **86.268305 ms meets 100 ms in this run**. The receipt is `diagnosticOnly=true`, `idleDiagnosticCompleted=true`, `originalReceiptComplete=true`, with no tasks/providers and `fullSustainedAcceptance=false`. Its `passed:true` records completed capture, not responsiveness acceptance. The passive long-frame attribution is still separate pending analysis; no cause, overhead subtraction or gain relative to earlier runs is asserted. The earlier5f 579.139597/97.398249 ms result remains unchanged at its original scope. Send stable issue IDs, exact revision, reproduction, expected/actual behavior and sanitized evidence as findings emerge. Continue independent permitted areas while fixes are prepared; change revisions only between closed batches. [Two-way testing protocol](LOCAL-TEST-LOOP.md) remains in force.
