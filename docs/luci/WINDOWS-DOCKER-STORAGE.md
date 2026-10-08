# Windows Docker storage confirmation

Latest reported native prerequisite state, 8 October 2026: Docker Desktop is **4.48.0 (207573)** and its live VHD remains on C:, with **3.49 GiB free** on that host volume. Relocation is awaiting a Windows unlock; the roomy E: backup is not evidence that the live engine has moved. No successful relocation or heavy-build readiness is claimed here. Windows packaging dependencies are reported present. A possible SDK registry-discovery mismatch remains unproven and does not establish a missing SDK or a need to reinstall it.

Docker Desktop's Linux filesystem can report hundreds of gigabytes free while its growing VHD sits on a nearly full Windows drive. Luci measures these separately. Windows execution and runtime preparation require fresh physical-storage observations as well as guest capacity. A stopped or unconfirmed engine does not prevent planning or editing project context.

The host Super Admin confirms the **existing full VHD/VHDX path** after checking Docker Desktop's current storage location. The confirmation explicitly covers both the active location and any pending relocation: they must be mounted local Windows volumes. Luci does not move the disk, change Docker settings or claim it automatically proved which VHD the engine uses. Network shares, raw-device paths and unmounted-volume paths are outside this adapter's supported layout.

Luci resolves the selected file to its actual local volume using a read-only attribute handle. It does not read VHD contents. The confirmation records the Windows user identity, selected file identity, engine ID, full Desktop release label and a fingerprint of the current user's Desktop settings. The settings path comes from the Windows KnownFolder API because worker profile environment variables are intentionally redirected. A settings, engine, Desktop version or selected-file change invalidates the confirmation; the operator reviews it again in Settings.

Physical headroom is the **lowest available space across every mounted writable local volume**, including removable drives and volumes mounted in folders. This is deliberately conservative: an unrelated nearly full C: drive or removable drive can limit new work even when the selected VHD is on E:. Host Settings identifies the limiting drive or exact folder mount for the authorized host operator. Project and team observations receive no private host path. An incomplete inventory is unavailable, not zero or unlimited capacity.

The gateway alone performs the Windows guest and physical-storage probes. It publishes an owner-private observation bound to the engine, daemon root and complete current confirmation. Workspace workers read that cache; they never fall back to WSL launches or native storage probes. Missing, mismatched or older-than-five-second observations refuse new heavy work. Before the first confirmation, periodic sampling skips WSL; explicit confirmation still checks the selected guest engine. This bounds probe duplication across active workspaces, but its native latency and CPU cost still need measurement.

The guest metric remains unchanged and separately labelled. Worker admission compares a storage-driver-aware estimate and reserve against both relevant capacities. Runtime preparation uses its own explicitly conservative working-space allowance; neither estimate guarantees the final size of arbitrary builds. Low space or stale observations pause admission of **new** heavy work; they do not terminate existing workers. The five-second observation expiry does not erase the saved operator confirmation.

## Read-only native probe acceptance

Run this on the native Windows test checkout after the relevant source revision is supplied. Use the isolated test session's existing private artifacts directory on a roomy drive. Keep the ordinary application and Docker data in place. This probe does not contact Docker, start workloads, publish a confirmation, alter existing state or move a VHD. It hashes Desktop settings and reads selected-file attributes and mounted-volume capacity.

From the checkout's control-plane directory in PowerShell, substitute the actual existing VHD location and a **fresh** receipt filename. Obtain the VHD location from the Desktop UI/native operator inspection; do not assume a default after relocation.

~~~powershell
$env:LUCI_WINDOWS_STORAGE_PROBE_VHD = '<actual existing local VHDX path>'
$env:LUCI_WINDOWS_STORAGE_PROBE_REPORT = '<isolated run artifacts>\storage-probe-new.json'
try {
    go test -count=1 -run '^TestWindowsLocalStorageReadOnlyProbe$' -v ./internal/resourcebudget
    if ($LASTEXITCODE -ne 0) { throw 'Native storage observation failed; preserve the result.' }
} finally {
    Remove-Item Env:LUCI_WINDOWS_STORAGE_PROBE_VHD -ErrorAction SilentlyContinue
    Remove-Item Env:LUCI_WINDOWS_STORAGE_PROBE_REPORT -ErrorAction SilentlyContinue
}
~~~

The test is skipped without the explicit VHD variable; a skipped run is not acceptance. The receipt reports the volume count, minimum available bytes, limiting-volume label and observed time. It omits SID, full path, volume GUID, file identity and settings hash. Keep the command's source revision and test output with the receipt. Native API calls may exceed their context deadline while Windows completes a filesystem request; the result is rejected after cancellation rather than presented as fresh.

This isolated probe does not establish engine-to-VHD binding or complete the setup workflow. Separately verify the actual Super Admin confirmation flow, denial to other roles, expiry after a harmless Desktop setting change, clear low-space guidance and successful admission after sufficient measured headroom. Do not manufacture low disk space or alter live VHD files to test these cases; use controlled tests for those failure scenarios.

The local tester confirmed Docker API Server.Platform.Name as **Docker Desktop 4.48.0 (207573)**; engine version 28.5.1 is a distinct value. Native volume enumeration and the complete confirmation/admission workflow remain unverified until their own native receipts are collected. The earlier short Windows process sampler receipt does not establish these new storage checks.

The [controlled storage receipt](evidence/2026-10-08-windows-storage-controlled.json) records 19 passing Linux race-test outcomes and successful Windows amd64 compilation of the resourcebudget and Docker-backend test executables, with exact source hashes. Compilation is not native execution. After the read-only native probe, the isolated cache/ACL fixtures can be checked without contacting Docker:

~~~powershell
go test -count=1 -run '^TestLocalStorageSharedCache|^TestWindowsLocalStorageObservationRejectsPublicACL$' -v ./internal/resourcebudget
~~~

Those fixtures create only temporary test files; they do not publish a real host confirmation or observe an existing VHD.


## Separate workspace processes and probe counts

The [controlled subprocess receipt](evidence/2026-10-08-windows-storage-multiprocess.json) extends the earlier storage checks. Three separate reader processes use isolated state directories and one shared private host-observation file in each of six phases. Across 360 reads, fresh observations are accepted; expired observations, a foreign engine, a changed confirmation revision and changed pins at the same revision are refused. Instrumented guest and host probe boundaries record zero worker invocations. The parent producer publishes three times. Recreating the producer backend simulates lost process-local state; this check does not claim an actual gateway process restart or native WSL execution.

Reproduce the controlled check from `control-plane` on either platform without Docker:

~~~powershell
go test -count=1 -run '^TestSharedWindowsStorageIsolatedProcesses$' -v ./internal/backend/dockerbe
~~~

Native multi-workspace measurement remains a separate acceptance gate. With the existing isolated test gateway and at least three separately identified worker processes, collect **process-start events**, not only periodic process samples, for storage-probe `wsl.exe` launches. Record each test-owned parent PID with its creation identity, the observation interval and any lost or unattributed events. Do not collect command lines, environment variables or unrelated process details. A trace with dropped events or ambiguous parent identities cannot establish an exact count. Record gateway launches separately from worker launches, then check that worker counts remain zero while the gateway updates the shared file and each worker consumes fresh observations.

Also verify that stopping only the isolated gateway leaves the workers' existing tasks intact but makes new heavy admissions wait once the shared observation is older than five seconds. Restart that same isolated gateway and verify admission can recover after fresh publication without reconfirming unchanged storage. Keep native startup/recovery timings and CPU usage with this receipt. The one-second whole-process sampler can measure resource pressure, but it can miss brief `wsl.exe` processes and therefore cannot prove probe deduplication by their sampled absence.
