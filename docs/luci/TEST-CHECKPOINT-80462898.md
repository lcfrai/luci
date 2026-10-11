# Local test checkpoint — 80462898

Exact product revision: **`80462898735a191cceaa4cb33e486b42b0a35f3c`**, available on `codex/luci-production-runs`. This is a usable source checkpoint for the continuing Windows test loop, not completion of the local product. Adopt it only between test batches; preserve earlier checkouts, state and receipts.

## Change and evidence

CLOUD-082 adds preparation of missing public Docker bases for project previews. Luci plans the declared bases without mutation, reserves shared capacity, rechecks current Deploy authority/source/allocation before each mutation, and pins acknowledged immutable images for the build. Interrupted pull acknowledgements remain charged and do not trigger a retry. No environment variable, credential name or dependency was added.

[Source evidence](evidence/cloud082-preview-bases/README.md) preserves the original cold-base failure, 186 initial race outcomes and all-package compilation. A later COPY-parser correction has its own original four failures and nine passing affected cases; the 186 run predates that correction. [Windows cases](evidence/cloud082-preview-bases/WINDOWS-HANDOFF.md) cover cold bases, multistage COPY, changed authority/source, interrupted acknowledgement and capacity refusal. Actual cold-image provisioning remains unverified.

The earlier exact `577f7a0f` package has an [actual Linux preview pass](evidence/cloud080-installed-preview-577f7a0f/README.md): reviewed source, a COPY-only calculator image, useful HTTP output, inspected serving limits and exact API Stop/lease release. It is a handwritten fixture, not generated application quality or a build RUN test. Its [runtime installer disk refusal](evidence/cloud081-installed-disk-577f7a0f/README.md) confirms no Docker mutation under insufficient disk; successful installation remains open.

Windows at `577f7a0f` independently passed native/portable packaging, 32 controlled backend top-level tests plus 83 subtests, 31 runtime UI fixtures, and LOCAL-024's original forced-parent-death/same-state reopen reproduction. External `LUCI_API` preservation and ordinary native close are still separate pending cases. Carry these results with their original revision rather than repeating unaffected checks.

## Retrieve and run safely on Windows

Use the existing task-local Node **24.19.0**, Go **1.26.3** and E: caches/temp. Native packaging also requires the already verified Rust/MSVC/SDK/WebView2 setup. From an existing repository, use a new worktree; do not switch a running acceptance checkout:

```powershell
git fetch origin codex/luci-production-runs
git worktree add --detach E:\Luci-local-acceptance\checkouts\80462898 80462898735a191cceaa4cb33e486b42b0a35f3c
Set-Location E:\Luci-local-acceptance\checkouts\80462898
$env:GOMAXPROCS = '2'
$env:CARGO_BUILD_JOBS = '1'
Push-Location desktop
npm ci
Pop-Location
.\scripts\build-release.ps1 -Version 0.5.0
# Add -Native for a separate native packaging/lifecycle batch.
```

Check exact source, `sourceDirty=false`, sizes and SHA256 in the release manifest. Extract to a new directory. In a task-local shell with unrelated inherited `LUCI_*` settings cleared, select private fresh directories and a free port:

```powershell
$runRoot = Join-Path 'E:\Luci-local-acceptance\runs' ([guid]::NewGuid().ToString('N'))
$env:LUCI_PROJECTS_DIR = Join-Path $runRoot 'projects'
$env:LUCI_SANDBOX_IMAGE = 'luci/sandbox:accept-' + [guid]::NewGuid().ToString('N')
# Run from the new portable extraction; verify 19750 is free first.
.\Start-Luci.ps1 -StateDir (Join-Path $runRoot 'state') -Port 19750
```

For native own-sidecar acceptance, use the [isolated native route](LOCAL-WINDOWS-ACCEPTANCE.md#build-and-run-the-matching-native-app), with `LUCI_API` absent and explicit fresh non-linked `LUCI_STATE_DIR`, `LUCI_PROJECTS_DIR` and WebView data. Confirm the effective state through `runtime_info`. The source acceptance launcher remains available for UI-only work; its clearing of inherited `LUCI_*` means it is not the unique-image installer route above. No change to user-wide settings or existing application data is needed.

## Next useful acceptance batch

1. Finish the current native external-API and ordinary-close cases before changing revisions.
2. Confirm fresh owner/setup and Docker's actual E: backing storage. Preserve unrelated images/containers. A runtime build still requires its existing 32 GiB build allowance plus 8 GiB reserve, and sufficient observed RAM/CPU; do not bypass refusals.
3. Use Prepare runtime with a unique unused tag. Record actual build limits, Cancel/Check outcome behavior, final source/image identity and settlement. Connect the chosen provider through supported sign-in without sending credentials in chat.
4. Take a real app through context, isolated implementation, reviewed integration, actual passing checks and useful preview. Exercise a warm multistage Dockerfile and a separately uncached declared public base; do not prune shared images to create a cold fixture. Verify exact Stop and retained history. Scripted model responses establish plumbing only.
5. Continue four-project context, queue controls, dependency/recovery and sustained resource/response checks while ordinary applications remain open. Send blocking findings immediately with exact revision, reproduction and evidence; related minor findings can be grouped.

## Gates still open

- **CLOUD-032:** the plain packaged `577f7a0f` run completed all 100 switches, 200 inputs and 100 resets, but switch p95 **497.324950 ms** fails the unchanged 300 ms target. Input p95 **87.794101 ms** meets 100 ms for that run. [All timing rows and cleanup](evidence/cloud032-idle-577f7a0f/README.md) remain; two same-start Chromium PID-1 zombies are explicit. Diagnostic instrumentation is not a latency improvement or Windows acceptance.
- **CLOUD-080/081/082:** successful actual runtime installation, build RUN resource behavior, cold preview preparation, interrupted-operation recovery and native Windows execution are not established by source fixtures or the earlier COPY-only preview.
- **LOCAL-024:** external API ownership and ordinary native close; original forced-parent-death and reopen are verified at their Windows scope.
- **CLOUD-077 and retained recovery gates:** the required old-response-after-Restart-acknowledgement ordering and remaining exact-authority/concurrent-charge recovery cases remain open. Earlier failed fixtures and charged leases are preserved.
- Real-provider result quality and the complete four-project workload/resource/response milestone remain open. This source checkpoint does not claim market leadership or a finished product.

No cloud billing, public deployment or hosted infrastructure prerequisite blocks this local batch. The later cloud phase remains part of the broader goal.
