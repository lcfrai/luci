# Windows source fix checkpoint — 4d659507

Exact source: [`4d6595072888bfa06fed0070d9c63b4cc1e6d1a9`](https://github.com/lcfrai/luci/commit/4d6595072888bfa06fed0070d9c63b4cc1e6d1a9), branch `codex/luci-production-runs`. This is CLOUD-056's source handoff, with a new controlled recovery regression and published evidence. **It is not a matching binary package or LOCAL TEST HANDOFF READY.** The latest verified Linux package remains the older edd0 checkpoint; it does not contain CLOUD-056.

There are no configuration or secret-name changes. Keep the [safe retrieval, setup, native build and four-project sequence](TEST-CHECKPOINT-67A5B7C7.md), [cancellation cases](TEST-CHECKPOINT-D20EBD29.md) and [terminal cases](TEST-CHECKPOINT-EDD0CB12.md), substituting this exact source. Preserve existing state/projects, previous evidence and shared Docker workloads. Do not launch below the local tester's established memory gates: 2GiB for source work, 2.5GiB for browser/native work. This publication has not measured Windows headroom.

Use task-local Node24.19.0/npm and Go1.26.3, with caches on a spacious volume. The tester's npm10.9.3 differs from the cloud validation's npm11.9.0. Create a fresh checkout; never reset a running or previously tested revision:

```powershell
$candidate = '4d6595072888bfa06fed0070d9c63b4cc1e6d1a9'
$checkout = Join-Path 'E:\Luci-local-acceptance\checkouts' ('4d659507-' + [guid]::NewGuid().ToString('N'))
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

Use disposable owners/members and projects inside the isolated acceptance state. No provider credentials or worker allocation are required for the access-display cases. Keep a terminal buffer if an already authorized fixture allocation is available; otherwise record that portion untested.

1. In a team workspace, retain a project draft and visible session/history. Revoke that member from another authorized fixture-owner session. At the next authoritative session-list403 `team_access_denied`, private workspace content must hide. This is about previously delivered content; backend read/execute authorization must remain independent.
2. Confirm a blocked explanation remains stable through repeated session polls, without a new membership lookup for each denial. Explicit Retry, focus and the existing periodic refresh must still revalidate. Transient verification failure must keep the view hidden and retain the draft.
3. Restore membership with changed project visibility. Membership alone must not expose old projects/sessions: fresh project and session catalogues must succeed first. Verify authorized content returns, excluded content stays hidden, and old responses cannot overwrite a changed workspace/account/allocation.
4. Switch among four distinct projects and teams, preserving scoped drafts and current permissions. Test confirmed membership removal separately from transient network failure, account401 and an unrelated403. Hidden mounted readers may still make server-authorized reads; blanket request suspension is not claimed.

[CLOUD-056 evidence](evidence/cloud056-authority-display-20261009/README.md) retains original counterexamples, initial383 affected passes/backend403 pass and the test-only TypeScript error. Final46 affected tests and TypeScript pass with432 stable repository inputs. Counts overlap; no actual-browser paint or native pass is inferred. The [new ordered recovery test](evidence/cloud038-ordered-charged-witness-20261009/README.md) passes one race-enabled production-HTTP fixture with a separate charged workspace; actual Docker ordering/concurrent-witness and CLOUD-039 refusal gates remain open.

## Known remaining gates

The [complete edd0 diagnostic](evidence/cloud032-edd0-await-boundaries-20261009/README.md) preserves all432 actions: switch p95 582.760219ms fails300ms; input76.477972ms meets100ms. No overhead is subtracted. Independent exact driver/controller closure is available; historical browser PID/start identities were not recorded for that diagnostic.

Fresh edd0 terminal attempt5 delivers final stdout and typed exit0, then browser error/close1006. Its UI correctly remains ended, but clean transport shutdown is unresolved under CLOUD-054; restart/second-exit acceptance was not reached. Original failed fixtures remain intact. Supported Stop releases that attempt's charge; attempt3's separately retained charge is not manually repaired. Two exited Chromium PID-1 zombies remain explicit in terminal closure evidence.

Full Windows four-project runtime/recovery/resource acceptance, installed access behavior, live-provider quality, report capture and responsiveness remain open. Send stable issue IDs, exact revision and actionable reproduction/evidence between batches; do not switch source underneath a running test.
