# Windows source checkpoint — edc742e8

Exact integrated source: **`edc742e8b6ce4216a1726e74cceef892663ce3d5`**, available on `codex/luci-production-runs`. This is the next focused test checkpoint, not a finished-local-product claim. Adopt it between closed test batches and retain earlier checkouts, state and evidence.

## What changed

CLOUD-080/082 adds **Cancel build** while the original preview Start request remains pending. It records intent against the exact live release revision, rechecks current Edit/Deploy authority after lock waits, and signals only the owned operation. It does not wait for the API build lifecycle lock, issue Docker cleanup, replay work or release unknown capacity. Stop remains separate for an acknowledged serving allocation.

The UI retains uncertain outcomes and requires an explicit subsequent Refresh before another action following an uncertain response. Late responses cannot cross project or authority boundaries. Controller Close joins preview publication for one five-second budget and seals remaining publishers. Reopening an inactive cancelling receipt shows unknown rather than polling indefinitely; the saved history and charge remain unchanged.

[Retrievable evidence](evidence/cloud080-preview-cancel/README.md) contains separate scoped results: 87 product race outcomes, 11 management outcomes, one registered HTTP/API/ledger race test, 66 frontend assertions, TypeScript success, and actual styled controlled-HTTP checks at 1440/390. A later narrow restart-observation follow-up reproduces two failures with two positive controls, then passes the same four cases. These are not a combined full-suite or installed cancellation pass. All 188 checksum-declared publication members were checked against staged Git bytes before the source commit. Final browser cleanup retains eight absent observed identities and two PID-1 Chromium zombies.

The matching Linux package and actual Docker cancellation check are being prepared separately. The earlier [80462898 package](evidence/package-80462898/README.md) and [warm multistage build/serve/ordinary-Stop pass](evidence/cloud082-installed-warm-80462898/README.md) remain valid at their original source; they do not verify this correction.

## Retrieve and start on Windows

There are **no configuration, credential, lockfile or runtime-image source changes** in this correction. Use task-local Node24.19.0, Go1.26.3 and E: caches/temp; native packaging also uses the previously verified Rust/MSVC/SDK/WebView2 setup.

```powershell
git fetch origin codex/luci-production-runs
git worktree add --detach E:\Luci-local-acceptance\checkouts\edc742e8 edc742e8b6ce4216a1726e74cceef892663ce3d5
Set-Location E:\Luci-local-acceptance\checkouts\edc742e8
$env:GOMAXPROCS = '2'
$env:CARGO_BUILD_JOBS = '1'
Push-Location desktop
npm ci
Pop-Location
.\scripts\build-release.ps1 -Version 0.5.0
# Add -Native for a matching native lifecycle/cancellation batch.
```

Verify exact HEAD, clean source, `sourceDirty=false`, archive hashes and sizes against the release manifest. Extract to a new directory. In a task-local shell with unrelated inherited `LUCI_*` settings cleared, use fresh isolated directories and a checked free port:

```powershell
$runRoot = Join-Path 'E:\Luci-local-acceptance\runs' ([guid]::NewGuid().ToString('N'))
$env:LUCI_PROJECTS_DIR = Join-Path $runRoot 'projects'
$env:LUCI_SANDBOX_IMAGE = 'luci/sandbox:accept-' + [guid]::NewGuid().ToString('N')
# From the fresh portable extraction; first verify 19750 is free.
.\Start-Luci.ps1 -StateDir (Join-Path $runRoot 'state') -Port 19750
```

For own-sidecar native testing use the [isolated native route](LOCAL-WINDOWS-ACCEPTANCE.md#build-and-run-the-matching-native-app), explicit fresh non-linked `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR` and WebView data, with `LUCI_API` absent. Confirm effective state through `runtime_info`. The explicit external-API case must preserve that independently owned service. Never use the disposable-runner-only installed test script on this personal host.

## Prioritized local acceptance

1. Carry LOCAL-024's verified577 forced-parent-death and same-state reopen scope forward. Finish its separate external-API ownership and ordinary-close cases; do not repeat unaffected completed checks merely because the source moved.
2. For a reviewed disposable preview, observe its exact active build, then Cancel build while Start is pending. Record the cancellation response separately from final build settlement. Verify current permission/stale revision refusals, independent project controls and no replacement/replay. Use [the focused Windows cases](evidence/cloud080-preview-cancel/product/WINDOWS-FOCUSED-CASES.md), including before-allocation, serving-health and close/reopen boundaries as separate cases. Unknown charges remain until a supported exact settlement path proves otherwise.
3. Complete actual runtime preparation under the unchanged memory/CPU/disk gates, using a unique unused image tag and the confirmed E: Docker backing storage. Preserve unrelated containers/images. Test installer Cancel/Check outcome independently of preview cancellation. The cloud host cannot meet the unchanged40GiB cold-install floor.
4. Connect the chosen provider through supported sign-in. Take a useful app through context, isolated implementation, reviewed integration, passing checks and preview; exercise amendments. A handwritten/scripted fixture does not verify model quality.
5. Continue the four-project functional/resource sequence: distinct durable context; queued/running/blocked attention and priority controls; dependency and reviewed integration; failure/cancel/restart; useful output checks; sustained owned-process RAM/CPU and whole-computer responsiveness. Preserve the existing acceptance budgets, all timing rows and partial failures.

CLOUD-032 still fails switching: the latest plain packaged577 run records497.324950ms p95 versus300ms; input87.794101ms meets100ms for that run. No latency improvement is claimed here. Cold pulls, unknown Docker-build recovery, CLOUD-077's old-response ordering, retained concurrent-charge/authority recovery cases, real-provider quality and full Windows/four-project acceptance remain open. No cloud-only billing/deployment prerequisite blocks this local test milestone.
