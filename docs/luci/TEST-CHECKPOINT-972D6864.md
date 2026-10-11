# Windows test checkpoint — 972d6864

Exact source: [`972d6864a30b742b12545c8e12f4ca3ae2faa884`](https://github.com/lcfrai/luci/commit/972d6864a30b742b12545c8e12f4ca3ae2faa884), branch `codex/luci-production-runs`. This follows [78665270](TEST-CHECKPOINT-78665270.md). Adopt it only after closing and recording the active 786 batch; keep that batch's fixtures and results bound to 786. **Not LOCAL TEST HANDOFF READY or release acceptance.** Windows execution of this revision remains unverified.

[Matching Linux package receipts](evidence/package-972d6864/README.md) bind a **55.468211-second clean build, exit 0**, **965 matching extracted files**, **347 licensed components with none missing**, and the original build log's **54 compact guide/index files / 367 rewritten links**. API binary SHA-256 is `ea4df58d5a8f6349ec258c82f35eb5f7e577a33aec1e33f88ab348e53b0b8431`; portable archive SHA-256 is `03340a8313890763de5596fcf416890d319e16d11d283ed7d51e32f1a939a193`. Archives remain publisher-held. This is retrievable source and a verified Linux package, not a downloadable Windows binary. Native packaging/execution at 972d and runtime-image/installed acceptance remain separately scoped.

## What changed and what was checked

No setup, dependency, credential or launcher change is required relative to 786. The source correction adds an advisory terminal-preparation hint and skips redundant store/persistence writes for unchanged session polls. Authority, admission, polling freshness and Restart acknowledgement remain required.

| Check | Exact supported result |
| --- | --- |
| [CLOUD-076 preparation](evidence/cloud076-attach-readiness-frontend-20261009/README.md) | Final frontend: **340 affected cases pass**. Final [backend](evidence/cloud076-attach-readiness-backend-20261009/README.md): **38 focused + 40 related outcomes pass** at their recorded source scopes. [All 950 captured backend inputs](evidence/cloud076-attach-readiness-integration-20261009/README.md) match integration; parity is not a rerun. Superseded pure-endpoint and unmounted-replacement drafts retain their failures. No installed/Windows result or explanation of the historical HTTP 409 follows. |
| [CLOUD-078 unchanged polls](evidence/cloud078-session-poll-noop-20261009/README.md) | Identical controlled cases: **27 pass/9 expected failures before → 36 pass after**. Fresh reads and meaningful row/order changes still publish; unchanged rows skip notifications and preference writes. No installed resource or latency improvement is measured. |
| [SX.2 contract repair](evidence/cloud076-integrated-validation-resume-20261011/README.md) | The unchanged seam test goes **23 pass/1 fail → 24 pass** after adding the same optional Session field to the mirrored contract. The equality assertion remains intact. |
| [CLOUD-079 test synchronization](evidence/cloud079-draft-recovery-hydration-20261011/README.md) | Test-only change waits for initial mock hydration and checks the exact retained edited scene. Unchanged isolated file **22/22**, corrected file **22/22**. The integrated failure's cause remains unconfirmed; no product defect or product fix was established. |
| [Full desktop TypeScript](evidence/cloud076078-final-typecheck-20261011/README.md) | Separate integrated working-tree check: **exit 0, 18.02 seconds, 1,229 unchanged input bindings**, Node 24.19.0. This ran before the source commit; it is not described as execution on a clean 972d checkout. |

The original interrupted combined frontend reporter remains **2,938 passed / 2 failed across 168 files**. Original Vitest/wrapper exit and the subsequent TypeScript invocation/result are missing; that invocation supplies no typecheck pass. The later targeted corrections and separate full typecheck are distinct evidence, not a new clean full-suite result. All 1,229 historical input hashes matched at the later resumption comparison; that is not an original end capture.

## Latest Windows baseline — reported at 786

The local tester reports a locked npm install, production TypeScript/Vite build and **full native build PASS, exit 0, from exact clean 78665270 source**. An **unsigned 62.74 MiB NSIS installer was generated but not installed**. The reported source-browser pass covers four UI projects with saved distinct purposes and successful revisits; Delta's Keep editing, Keep draft and continue, and recovery; cross-project palette navigation; and the 390 px drawer's four destinations plus usable Projects focus after dirty-dialog Escape. These are relayed local results, not independently inspected cloud executions, and do not transfer automatically to 972d. **Twelve controlled recovery checks remain pending.**

LOCAL-022 is a restored environment prerequisite: the tester reports the original Docker engine healthy with **107 containers, 155 volumes, 178 unique images and the same four running containers unchanged**. Its recovery cause remains unestablished; state, VHD and shared workloads are preserved. Earlier recovered headroom was reported as about 15.94 GB available RAM and 35.5% commit; recheck current resources before new work. LOCAL-023 remains a P2 naming gap: the tester's authoritative 7 October local instructions require **Discover, Design, Develop, Deploy**; the Define/Describe mismatch is not resolved by this checkpoint.

## Retrieve and launch an isolated source run

Use a dedicated shell and fresh E: paths. Keep the **40 GiB floor**, verify Docker Linux/backing-volume space, RAM and commit, and preserve shared work. Activate task-local **Node 24.19.0 / Go 1.26.3** and record npm. No new settings or secrets.

```powershell
$ErrorActionPreference = 'Stop'
$candidate = '972d6864a30b742b12545c8e12f4ca3ae2faa884'
if ((node --version).Trim() -cne 'v24.19.0') { throw 'Activate Node 24.19.0' }
if ((go version) -notmatch '^go version go1\.26\.3 ') { throw 'Activate Go 1.26.3' }
npm.cmd --version
$checkout = Join-Path 'E:\Luci-local-acceptance\checkouts' ('972d6864-' + [guid]::NewGuid().ToString('N'))
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

Choose distinct free loopback ports with the [isolated procedure](LOCAL-WINDOWS-ACCEPTANCE.md#retrieve-the-exact-candidate-and-start-safely), then:

```powershell
.\scripts\start-local-acceptance.ps1 -AcceptanceRoot 'E:\Luci-local-acceptance\runs' -Port $apiPort -FrontendPort $webPort
```

The launcher checks ports, creates fresh state and temporarily clears inherited `LUCI_*`. `-PrepareOnly` does not launch. **Continue to projects** needs no provider; shells require runtime, authority and admission. Keep bootstrap/state private. Ctrl+C closes the run; use documented same-state reopen steps.

For the native route, first satisfy resource and Rust/MSVC/SDK/WebView2 prerequisites:

```powershell
pwsh -NoProfile -File .\scripts\build-release.ps1 -Version 0.5.0 -Native
if ($LASTEXITCODE) { throw 'Retain failed native build output' }
```

Require exact-source manifest and matching app/sidecar/resources. Follow [isolated native launch](LOCAL-WINDOWS-ACCEPTANCE.md#build-and-run-the-matching-native-app): fresh non-linked state/project/WebView paths, `LUCI_API` absent for own-sidecar mode, and authoritative `runtime_info`. Installer generation is not installation or execution.

## Focused retests and remaining gates

1. **Preparation to attach:** while this API prepares an exact allocation, `attachReady:false` shows preparing without opening a ticket/socket or consuming the retry clock. A recorded endpoint gives true and allows an otherwise eligible attach even while status is starting. Omission is unknown, including legacy/untracked rows; it must retain the existing recovery path. True is neither health nor authority.
2. **Controller and mount recovery:** reopen the API with an existing backend allocation whose endpoint was not recorded; omission must permit backend recovery. Unmount a preparation wait, deliver a ready replacement generation, then remount within retention: no background opening while unmounted; retain terminal output and attach the current generation once. Merely hiding a mounted pane does not test this boundary. Existing same-generation open streams remain intact; expired/reset entries and denied/refreshing authority cannot revive through a hint.
3. **Restart and terminal fences:** exercise warm and cold explicit Restart with one successful acknowledgement, a new generation and preserved session/project identity. An early ready poll must not bypass the cold ACK fence. Preserve typed exits 0/7, normal Close 1000 and natural exact-charge releases with no extra post-Exit connection attempts. The still-open ordering case requires the old list response created before Restart to arrive **after** its ACK. An early gate release, pending Restart or retained charge leaves that case incomplete; preserve failed fixtures.
4. **Poll freshness and real changes:** an unchanged two-second poll still performs its fresh read/good-tick work but causes no store notification or preference write. Addition/removal, status, allocation, timestamps, readiness and per-worktree order changes remain visible. Pending Kill/Forget must stay protected from intervening polls; the first authoritative empty index still publishes once. Use bounded owned instrumentation if measuring writes; visual quiet alone does not establish this behavior or a performance gain.
5. **Draft, authority and keyboard continuity:** after initial sketch hydration, edit, leave/remount and restore explicitly; retain the edited scene, require no autosave, and keep account/workspace/project drafts separate. Repeat relevant project navigation/focus and denial/revocation checks. C079 changed only the fixture; preserve any actual UI failure independently.

Complete the twelve pending recovery checks with their actual result scopes. Fresh unknown-Create and acknowledged-worker/controller recovery must preserve history/output, avoid replay, and release only the settled owned allocation under current authority; do not edit private ledgers or repair historical fixtures manually. Do not run exclusive-daemon Linux helpers against the shared Windows Docker inventory.

Remaining acceptance includes exact native execution, full warm ordering, retained recovery, authority/draft/keyboard behavior, four-project workflow and resources, real-provider quality and idea-to-live completion. Keep the **300 ms project-switch / 100 ms input p95 targets** and full-action timing records; the historical 29931afc switch p95 **494.921816 ms** failed. Source checks, package generation, fewer state writes and the reported 786 browser successes do not close these gates. The current [quality worklist](QUALITY-WORKLIST.md) owns issue states; record each run's exact source, artifact, environment, outcomes and ownership-scoped cleanup.
