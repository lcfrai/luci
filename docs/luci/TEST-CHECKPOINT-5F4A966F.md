# Windows source fix checkpoint — 5f4a966f

Exact source: [`5f4a966f6d70264c92ee2176dde912e1b43fc3c7`](https://github.com/lcfrai/luci/commit/5f4a966f6d70264c92ee2176dde912e1b43fc3c7), branch `codex/luci-production-runs`. This source integrates CLOUD-054's upstream WebSocket close correction and CLOUD-060's private-dialog access barrier, retaining the earlier CLOUD-056 workspace correction. The matching clean Linux package has961 verified members/347 licence records and no missing licences. Its portable archive SHA256 is `bc7d8b76b76d3c291c651c3f46423f75f7e63f31f49ddde59372fe6de97e6290`; binary SHA256 `5d1404766d21fe74eac4f87fc4ec2177b3bb3bca2bee0347afb21eaefb25adf1`. These are cloud build identities, not independently retrieved Windows binaries. The accessible source is the exact Git revision. **This checkpoint is not full LOCAL TEST HANDOFF READY.** Matching runtime760 exports and the first clean terminal exit/Close are verified; full Restart failed the unchanged20-second status wait (CLOUD-063). Actual desktop/phone Team-access/settings denial and draft restoration pass (CLOUD-056/060); native scope remains open.

There are no configuration or secret-name changes. Keep the [safe retrieval, setup, native build and four-project sequence](TEST-CHECKPOINT-67A5B7C7.md), [cancellation cases](TEST-CHECKPOINT-D20EBD29.md) and [terminal cases](TEST-CHECKPOINT-EDD0CB12.md), substituting this exact source. Preserve existing state/projects, previous evidence and shared Docker workloads. Do not launch below the local tester's established memory gates: 2GiB for source work, 2.5GiB for browser/native work. This publication has not measured Windows headroom.

Use task-local Node24.19.0/npm and Go1.26.3, with caches on a spacious volume. The tester's npm10.9.3 differs from the cloud validation's npm11.9.0. Create a fresh checkout; never reset a running or previously tested revision:

```powershell
$candidate = '5f4a966f6d70264c92ee2176dde912e1b43fc3c7'
$checkout = Join-Path 'E:\Luci-local-acceptance\checkouts' ('5f4a966f-' + [guid]::NewGuid().ToString('N'))
git clone --branch codex/luci-production-runs https://github.com/lcfrai/luci.git $checkout
if ($LASTEXITCODE -ne 0) { throw 'Clone failed' }
git -C $checkout checkout --detach $candidate
if ($LASTEXITCODE -ne 0) { throw 'Exact checkout failed' }
if ((git -C $checkout rev-parse HEAD) -ne $candidate) { throw 'Revision mismatch' }
if (git -C $checkout status --porcelain) { throw 'Checkout is not clean' }
```

After activating the supported task-local runtime, run locked `npm ci` in `desktop`. Use `scripts/start-local-acceptance.ps1 -AcceptanceRoot E:\Luci-local-acceptance\runs -Port <free> -FrontendPort <distinct-free>` from the checkout; it creates fresh private state/projects. The documented `-PrepareOnly` path is available without launching the app. Never bypass the disposable-runner restriction in `test-installed-windows.ps1`.

For a native build, use `scripts/build-release.ps1 -Version 0.5.0 -Native` after resource admission. The release manifest must name the exact candidate with `sourceDirty=false`. Keep the entire native/sidecar/resource tree together. Launch only with fresh nonlinked `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR`, `WEBVIEW2_USER_DATA_FOLDER`, no inherited `LUCI_API`, and the free API address returned in `runtime_info.apiUrl`. Do not install the generated NSIS artifact over personal application data for this test.

## Focused retest

Use fresh disposable owners/members and four projects in the isolated state. No provider credentials or worker allocation are required for access-display checks.

1. Open Team access or a private reviewed-context/task dialog at1440px and390px. Keep a draft. Revoke the fixture member from a separate authorized owner session. At fresh session-list403 `team_access_denied`, both ordinary private panes and body-portaled dialogs must hide. Retry must remain usable by pointer/keyboard, without hidden modal focus/pointer/scroll locks.
2. Restore membership. Cached read permission alone must not reopen a suspended project review: fresh project/session catalogues and exact current project read authority must settle. Exercise held/failed permission reads, remove a project grant, cancel the review, and change account/project. No stale private contents may reappear. Hidden state is deliberately retained, not erased.
3. Restore exact authority and confirm drafts return. Finish a save while hidden and restore access: busy state must settle without replaying the save. Public sign-in/join/recovery remain usable. Bulk cancel review must check every distinct selected project, keep scope clear and make no mutation until its explicit action.
4. For terminal transport, build/install a runtime from this source, including the updated sandboxd vendor. An older cached image does not verify this correction. In an authorized disposable shell session, check final output plus exit0, one clean WebSocket close1000 through peer acknowledgement, ended UI without reconnect, same-ID restart and exit7. Nonzero exit remains failure evidence, not successful work. Use supported Stop and inspect exact container/lease settlement afterward. Keep stderr/raw terminal contents private and publish only reviewed disposable fixture evidence.

The new raw CI regression covers exit0/exit7/overload Close→ACK→EOF; all15 integrated proxy race outcomes pass, and both Go modules compile all packages. The external private-dialog batches have580 latest affected assertions/27files and TypeScript0; actual repository new tests21pass overlap. These counts are controlled source evidence, not Windows or full-runtime acceptance.

## Known remaining gates

CLOUD-032 response thresholds remain300ms switching/100ms typing p95 with unchanged workload. The latest full timing captures still fail switching; no source fix or settled-control measurement is credited as a speedup. Full four-project loaded/settled workflow/resource measurements, real-provider output quality, original-review-before-first-observer recovery with another charged Docker witness, and native Windows acceptance remain open.

Old failed terminal fixtures and retained charged reservations remain untouched. Source-controlled recovery cases and exact manual cleanup of a failed fixture are separate from supported product release of its capacity. Prior closure reports explicitly retain PID-1 Chromium zombies and unknown observations where applicable; no whole-machine zero-process assertion.

Send stable issue IDs, exact revision and actionable reproduction/evidence between batches. Do not switch source underneath a running test. The [rolling coordination protocol](LOCAL-TEST-LOOP.md) and [four-project capacity procedure](SWARM-CAPACITY-ACCEPTANCE.md) remain authoritative.
