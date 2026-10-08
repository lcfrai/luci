# Installed four-project resource observations: 8f78804f

The clean installed Linux package at `8f78804ff318ab744226f139df5998cbd6e0d0ff` **passed the controlled four-project workflow**. Four actual Docker/Codex tasks exited naturally and each exact result was reviewed and integrated. The observer recorded **294 samples over 295.83 seconds**. One worker was configured to run at a time; four ongoing projects are not four concurrent agents.

The [sanitized receipt](2026-10-08-8f78804f-installed-swarm-resources.json) contains the full numerical timeline, worker aliases, gaps and hashes linking the private observations to the exact workflow result, package archive and accepted sampler. The [main workflow receipt](2026-10-08-8f78804f-installed-swarm.md) records execution and lifecycle checks. This resource report does not claim the later CLOUD-009 recovery implementation was included in that package.

| Observation | Measured result |
| --- | --- |
| Maximum simultaneous browser + control-plane summed RSS | 1.09 GiB |
| Maximum observed driver-family summed RSS | 1.27 GiB |
| Maximum live owned processes | 12 |
| Worker cgroup memory peaks, in worker order | 46.16 / 45.65 / 54.29 / 46.28 MiB |
| Configured limit per worker | 1 GiB RAM, one CPU |
| Minimum whole-host available RAM | 16.59 GiB |
| Minimum root/workspace filesystem available space | 1.99 GiB |
| Sample interval median / p95 / maximum | 1.00 / 1.66 / 2.61 seconds |
| Final live owned-process count | 0 |

The observer interval ran from 10:15:24.29Z to 10:20:20.12Z on 8 October 2026. The workflow itself reported 276.837 seconds; observer startup and post-exit settlement make its interval longer. Settlement after the driver exits is not idle Luci with the app still open.

Process RSS sums overlap shared pages. Browser and API role peaks must not be added separately. Container cgroup memory is a different measurement and must not be added to process RSS. Daemon and shared-kernel overhead are unassigned. Whole-host memory, CPU and filesystem observations include unrelated activity. Root and workspace use the same filesystem, so their free-space values must not be summed; sequential readings can differ during writes.

Four initial samples lacked the exact Docker ownership identifier. Some process I/O was unavailable in 19 samples. Worker memory statistics were available in 11 / 13 / 11 / 11 samples; the larger per-worker observation counts also include created or stopped containers without running memory statistics. Short-lived processes and events between samples may be missed, and process counter deltas cover observed intervals only.

No coordinated compiler work overlapped this run. Source edits continued during its exclusive Docker/performance window. That coordination record is not proof that the shared host had no unrelated activity; the observer deliberately did not collect unrelated process details.

The controlled provider made no external model calls. This result verifies installed mechanics at the recorded load. It does not measure model reasoning quality, UI input latency, project-switch latency, other-application responsiveness or native Windows behavior. The roughly five-minute interval does not finish the longer installed or native soak gates.

## Remaining Linux measurements

The earlier ten-minute **controlled process-backend** capacity check remains valid for its recorded synthetic admission inputs. It does not replace these outstanding installed Docker/browser measurements:

1. Capture at least 60 seconds of warmed idle Luci with the control plane and browser open before worker allocation, then a distinct 60-second settle phase with Luci still open after work finishes. Use exact process creation identities and the same application/package configuration. The current trace cannot reconstruct that baseline or an idle-to-load memory increase.
2. Run the installed Docker/browser mixed workload for at least ten minutes across four ongoing projects. Keep actual worker concurrency, enforced limits, queued reasons and every workload phase explicit. Include builds/tests, bounded substantial terminal output, context amendment, switching between projects, failure recovery and restart. The current controlled tasks are short and use one execution slot.
3. Collect timestamped input-to-visible-update and project-switch measurements during idle and loaded phases, with enough recorded events to report p50/p95/p99 and long stalls. Record event count, rendering/automation method and unavailable samples. Successful browser assertions and one-second process sampling do not provide these latency values.
4. Measure sampler overhead separately and record exact probe invocations where brief subprocess counts matter. Resource snapshots alone cannot establish that a short-lived probe never ran. Keep Docker cgroup memory, browser/control-plane process memory and unassigned host overhead distinct throughout.

These are measurements to perform, not results inferred from this receipt. Native Windows still needs its separate thirty-minute workload, normal browser/editor activity, baseline, interaction measurements and Docker/WSL storage observations. Candidate responsiveness thresholds in [capacity acceptance](../SWARM-CAPACITY-ACCEPTANCE.md) remain unverified on Dylan's workstation.

Reproduction helpers are in the [tracked resource procedure](../../../scripts/acceptance/four-project-queue/RESOURCE-MEASUREMENT.md). Earlier successful and failed receipts remain separate evidence; this run does not rewrite them.
