# Runtime and preview source checkpoint — 11 October 2026

Use the exact immutable revision announced with this document on
`codex/luci-production-runs`, only between test batches. This is an integrated
**source checkpoint for continued acceptance**, not the finished local milestone.
The cloud session does not expose a cross-thread messaging tool; the local
thread's authorized heartbeat can read the accompanying checkpoint notice.

## What changed

CLOUD-080 routes both local preview entry points through shared machine capacity,
current Edit/Deploy authority, separate image-build/serving receipts, exact source
identity, enforced execution limits and reviewed cleanup. There is no direct CLI
build/start fallback when those controls are unavailable. Stop/restart observes
and controls the exact retained allocation; a replacement does not inherit stale
permission or capacity.

CLOUD-081 replaces the unbudgeted runtime CLI build with a trusted, shared-admission
installer. It preserves the existing **32 GiB build allowance plus 8 GiB reserve**,
checks current administrator access, supports Cancel and explicit Check outcome,
and retains ambiguous outcomes without rebuilding or retagging them. Only the
bundled source and two declared base images are allowed. A configured tag that
already points at a different image is refused. Do not retag the old October
image or treat it as this candidate's runtime.

The checkpoint includes LOCAL-023 naming and LOCAL-024 Windows parent-death work.
LOCAL-024 has nine real Windows kernel-test passes and one intentional ignored
fixture at `2416135f`; its original native driver-induced termination/reopen and
external API retests are still separate. No environment variable or credential
name was added. The new optional setup APIs are documented in [API.md](API.md).

## Evidence

[Integrated source capsule](evidence/cloud080081-integrated-20261011/README.md):
304 final race outcomes across 89 top-level tests, no failures/skips; all control-plane
packages compile; nine cold-demand outcomes; one real bundled-source/read-only
contract; one invited-member setup-route regression. Counts from earlier staged
batches overlap. The final race run includes the bundled-source and Linux build-wire
regressions. Both original invalid SecurityOpt requests and an obsolete fixture
assertion are retained; build resource limits remain enforced. Earlier all-package
compilation predates that narrow option correction. The first all-package compile failed for an
uncached locked module under offline mode; its verified download and passing
second compile are retained. No lockfile changed.

[UI evidence](evidence/cloud081-runtime-install-ui-20261011/UI-VALIDATION.md):
31 affected tests and full TypeScript pass, with the original timer-test harness
failure retained. [Backend protocol regressions](evidence/cloud080-backend-20261011/README.md)
and [adapter follow-up](evidence/cloud080-label-followup-20261011/README.md)
retain the original failures. These are controlled fixtures and source checks,
not a completed installed build or performance run.

## Windows preparation and startup

Continue using the tester's task-local Node **24.19.0**, Go **1.26.3**, npm lockfile,
E: caches/temp, fresh private state/projects and a free loopback port. A native
build additionally uses the previously verified Rust/MSVC/SDK/WebView2 route.
Preserve all existing projects, normal Luci state and unrelated Docker workloads.
Measure both Docker guest space and the actual E: backing volume, plus physical
RAM/commit. Do not lower admission thresholds to make a workload run.

From the pinned isolated source checkout:

```powershell
$env:GOMAXPROCS = '2'
$env:CARGO_BUILD_JOBS = '1'
Push-Location desktop
npm ci
Pop-Location
.\scripts\build-release.ps1 -Version 0.5.0
# Add -Native only for the separate native build/lifecycle batch.
```

Check the release manifest's source revision, `sourceDirty=false`, and each
artifact's size/SHA256 before extraction. Use a new extraction directory on E:.
For portable acceptance, use a dedicated shell with unrelated inherited
`LUCI_*` settings cleared in that shell, then select fresh GUID paths and a
unique **nonexistent** image tag:

```powershell
$runRoot = Join-Path 'E:\Luci-local-acceptance\runs' ([guid]::NewGuid().ToString('N'))
$env:LUCI_PROJECTS_DIR = Join-Path $runRoot 'projects'
$env:LUCI_SANDBOX_IMAGE = 'luci/sandbox:accept-' + [guid]::NewGuid().ToString('N')
# From the freshly extracted package; check that this port is free first.
.\Start-Luci.ps1 -StateDir (Join-Path $runRoot 'state') -Port 19750
```

The source acceptance launcher remains safe for source/UI testing and clears
inherited `LUCI_*`; it does **not** retain a custom image override. Use the explicit
portable route above for the unique-image installation test. Keep private tokens,
provider credentials, Docker storage confirmations and raw private fixtures out
of the public handoff. No provider secret value should be sent in chat.

## Prioritized next local checks

1. Finish the ongoing LOCAL-024 native original reproduction/reopen/external-API
   batch on its current pin before adopting this successor.
2. Build and start this exact source with fresh state. Create the owner; verify
   Git/Docker readiness and the existing real backing-volume confirmation.
   Inspect normal insufficient-capacity refusal before doing any heavy work.
3. With sufficient actual headroom, use **Prepare runtime**. Record configured
   limits, host/guest capacity, actual pull/build/settlement and the exact final
   image ID/source digest. Check that Cancel stays responsive and that a lost
   acknowledgement remains unknown instead of silently retrying. No unrelated
   tag/container/volume should change. Do not conflate client closure with daemon
   settlement. Use a disposable separate run for interruption coverage.
4. On a successful install, connect the user's chosen provider through supported
   sign-in. Exercise real plan→isolated build→review→verified checks→preview. A
   scripted provider can establish transport plumbing only. Use a Dockerfile
   whose immutable base is already available; arbitrary cold project-base
   provisioning is still an explicit gap. Verify the preview's useful action,
   exact checked source and Stop/restart authority/capacity behavior.
5. Continue four-project context, controls, dependency/recovery and sustained
   whole-computer measurements. Preserve user apps. Keep the unchanged 300 ms
   switch/100 ms input targets and all failed timing rows.

## Remaining limitations and gates

- No actual installer/preview Docker run is claimed for this checkpoint. This
  cloud filesystem has about 5 GiB free and cannot meet the unchanged 40 GiB cold
  installer requirement; controlled and read-only checks were completed here.
- Build RUN containers have CPU/memory/swap limits. Daemon download/image work
  shares reserved capacity but is not individually cgroup-capped. Measure its
  real Windows host/VM cost before accepting responsiveness.
- Unknown build transport retains capacity. A settled build with lost tag ACK can
  be inspected without mutation; automatic recovery of every ambiguous running
  build remains open. Existing failed fixtures/charges are unchanged.
- Docker has no atomic tag CAS against unrelated privileged clients. Luci
  serializes installer jobs and refuses an observed differing tag.
- Arbitrary missing preview-base images and unsupported Dockerfile syntax are
  refused before mutation; their provisioning support is not complete.
- CLOUD-032 remains failed at the last matching 972d idle measurement:
  switch p95 400.974902 ms versus 300 ms; input 79.123043 ms versus 100 ms.
  This checkpoint has no new latency measurement or improvement claim.
- CLOUD-077 warm old-response-after-restart-ACK ordering, Windows installed
  execution, live-model quality and the broader local acceptance ledger remain
  open. Earlier Windows results retain their exact original revisions/scopes.
