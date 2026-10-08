# Reported native Windows observer check

The native tester reports **nine controlled checks passed**, followed by an **8.017-second capture with eight samples** of a newly owned PowerShell root and its exact descendant. Physical memory, commit memory, owned working set and the E: disk measurement were present in all eight samples. CPU and I/O deltas followed the initial baseline. The output ACL was restricted to the current user; both observed process identities subsequently exited.

This is a report from the Windows tester, not a rerun or raw-artifact inspection by the cloud development session. The [machine-readable receipt](2026-10-08-windows-owned-observer-native-reported.json) preserves the reported private artifact locations, the source-checkout caveat and the limits of the result. No native artifact hashes were supplied. The checkout began at `9898cf5d`; a separate Git fixture later changed its HEAD to `8f4deb8a`. The tester reported unchanged Go and sampler blobs, which this receipt does not independently verify.

The capture exercises native host/process/disk readers and output ACL creation. Docker was not configured for this capture. It does **not** validate Docker named-pipe statistics, Luci or WebView memory, a four-project workload, sustained responsiveness, or Docker's physical backing-disk admission. Those remain separate acceptance checks.

Reproduce or extend the capture using the [resource measurement instructions](../../../scripts/acceptance/four-project-queue/RESOURCE-MEASUREMENT.md). Keep the earlier [Linux controlled receipt](2026-10-08-windows-owned-observer-controlled.json) separate: its source hashes and nine simulated checks are evidence from that earlier run, not hashes of the native tester's output.
