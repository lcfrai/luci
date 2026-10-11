# Source checkpoint 7c998624 — focused Windows retest

This is an actionable **source checkpoint**, not LOCAL TEST HANDOFF READY for the entire product. Scope contract b3ecb011 remains controlling; its audit gate is lifted. The working objective is active; this does not reactivate the blocked platform goal. No provider, live microVM, latency or full delivery acceptance is claimed.

## Exact source and retrieval

Repository: https://github.com/lcfrai/luci, branch `codex/luci-production-runs`.

Product/source pin: **`7c998624d94720e84c8cef0236d0dafbe17a535b`**. Use authenticated Git access. Retrieve into a new isolated checkout on E: or F:, switching only between test batches. Preserve original source, state, evidence and shared Docker workloads.

```powershell
git clone --no-checkout https://github.com/lcfrai/luci.git E:\Luci-local-acceptance\checkouts\7c998624
git -C E:\Luci-local-acceptance\checkouts\7c998624 checkout --detach 7c998624d94720e84c8cef0236d0dafbe17a535b
git -C E:\Luci-local-acceptance\checkouts\7c998624 status --short
```

The clean Linux package build and actual four-project HTTP policy/restart smoke now pass. [Bound package receipt, exact archives and retained helper failures](evidence/checkpoint-7c998624-package/README.md) distinguish these checks from provider/browser/Windows acceptance. The archives remain in the cloud workspace; source is accessible in Git, not a published binary download. Do not reuse an older package as this revision.

## Setup and commands

No new user configuration, database migration, runtime image sources, native-sidecar sources, acceptance launcher or Windows release-build script changes relative to577f7a0f. This does not establish provenance for an old installed image: keep using a uniquely tagged matching runtime or a separately verified existing image. The repository's bundled upstream runtime source is unchanged. CLOUD-089's reported Windows result can be carried forward at its original source scope.

Prerequisites remain Git, Node24.19.0, npm10.9.3, Go1.26.3 and healthy Docker Desktop Linux engine. Native build additionally needs the already-verified Rust/MSVC/Windows SDK/WebView2 tools. Keep task-local caches/temp on a roomy volume and bounded build concurrency. Use actual current available host RAM and physical backing-volume space; do not substitute guest `df` for host space.

```powershell
Set-Location E:\Luci-local-acceptance\checkouts\7c998624
$env:GOMAXPROCS = '2'
$env:GOFLAGS = '-p=1'
npm --prefix desktop ci
powershell -NoProfile -File scripts/start-local-acceptance.ps1 -AcceptanceRoot E:\Luci-local-acceptance\runs -Port 19760 -FrontendPort 15230
```

Choose other free ports if occupied. The supported launcher creates private GUID-named state/projects, sets `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR`, `LUCI_ADDR` for only this process, and restores inherited configuration afterward. Use `-PrepareOnly` for preflight only. Never bypass the disposable-runner guard in `test-installed-windows.ps1`.

For the documented native/portable build:

```powershell
powershell -NoProfile -File scripts/build-release.ps1 -Version 0.5.0 -Native
```

This stages required guides/resources. Run with a fresh explicit `LUCI_STATE_DIR`/`LUCI_PROJECTS_DIR` and free `LUCI_ADDR` for owned-sidecar acceptance, following [the Windows guide](LOCAL-WINDOWS-ACCEPTANCE.md). Set `LUCI_API` only for the separate external-API scenario. No secret values belong in logs or this handoff. Configure the selected supported provider/sign-in through Luci; no new credential names are introduced here.

## Changes and existing checks

- **CLOUD-087, d5bf1f26:** persist the bounded actual sample/decision projection on a new runtime-install refusal, survive GET/restart, show read-only "Why preparation paused" advice. Unavailable operands stay absent; observed zero is explicit. No probe on disclosure, predicate change, replay or invented original cause. [Evidence](evidence/cloud087-admission-observation/README.md): selected API/product race22top+23sub, UI13 assertions; not a clean whole compiler-input closure.
- **CLOUD-089, 3ba9efed:** exact never-started Created objects are not live workload demand; uncertain/live objects and pending owned reservations remain conservative. [Evidence](evidence/cloud089-created-observation/README.md):11top+26sub; one opt-in skip. Local tester separately verifies exact production observer against shared Windows Docker and those controlled memory cases. No install/model pass.
- **CLOUD-085/086, 8805c8c6 + d19981fd:** durable project draft policy, actor-scoped exact continuation identities, current-authority/context binding, existing typed run/prepare/queue operations, durable shared intents and conservative compatible reconciliation. [Backend](evidence/cloud085-coordinator/backend/README.md): regular40top+42sub; final race24producttop+24sub+one management test. Counts overlap; API package compile selected no runtime test. [Original shared-intent failure](evidence/cloud086-shared-intents/backend/README.md) and [race addendum](evidence/cloud086-shared-intents/race-addendum/README.md) are retained.
- **Conversation/team controls:** policy uncertainty survives remount/permission churn without replay; rejected reads hide private cached team content; stale child reads cannot overwrite newer parent context. [Final parent/UI](evidence/cloud085-observation-followup/README.md):201 assertions/four files and TypeScript0; [team read-denial](evidence/cloud086-team-intents-read-denial/README.md):31 assertions. Earlier counts overlap. No actual browser/native/latency claim.
- **CLOUD-084:** committed guarded allocation/transfer library, not yet selected or advertised as a functioning microVM backend. [Evidence](evidence/cloud084-allocation-boundary/README.md). No installation/provisioning/setup delta.

## Prioritized Windows checks

1. **New refusal diagnosis and guarded runtime installation.** Preserve original577 job untouched. On new isolated state confirm real E: Docker backing storage, run normal readiness, then request installation only under unchanged current resource policy. Record exact new job/refusal projection or acknowledged build/caps/settlement. A refusal is not an installation pass. Expanding diagnosis, changing setup tabs and reopening must not retry or affect another workspace. A successful install remains distinct from connected provider/build success.
2. **Conversation and team UI.** Four projects with unique canonical context; per-project policy defaults remain review/manual. Test stale policy/context CAS, unresolved questions, permission revocation, uncertain response readback, project switching and restart. Team intent conflict must remain explicit; compatible draft reconciliation alone does not prove team coding/delivery. Two authenticated people must not see each other's private continuation records. Frontend continuation-execution controls are a subsequent slice, not in this source pin.
3. **Actual local upstream path.** After a verified matching runtime and supported in-container provider/sign-in, run the approved StudioSchedule workflow through Docker. Bind actual harness/model, source/context, changes, checks, review, preview and deployment status. No host-CLI glue or scripted provider is real-model acceptance.
4. **Remaining acceptance.** Keep full conversational amendment-to-deployment, two-Lucis live coordination, microVM per-task fan-out/restart/teardown, old-response ordering/concurrent retained charges and whole-computer four-project resource/latency gates open. Unchanged targets: switch p95<300ms, input p95<100ms, interaction p99<1s. CLOUD-032 remains failed; these source changes carry no speedup claim.

LOCAL-024's577 parent-death/reopen/external-API/ordinary-close pass retains its precise original scope; unchanged native bytes do not need an unconditional repeat. Any new failure should carry stable issue ID, exact revision, reproduction, expected/actual, evidence and owned cleanup. We continue fixes concurrently; adopt successor revisions only between batches.
