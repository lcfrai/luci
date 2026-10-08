# Installed four-project resource observations: 2819ce9e

The installed controlled workflow **passed across four projects**, with four actual Docker/Codex tasks exiting naturally, shared capacity released before human integration, and four reviewed results integrated. The observer recorded **445 samples over 450.37 seconds**. This run allowed **one worker at a time**; it is not evidence of four simultaneous live-model builds.

The [sanitized receipt](2026-10-08-2819ce9e-installed-swarm-resources.json) includes the full numerical timeline, worker aliases, measurement gaps and hashes binding the private raw observations to the workflow result and accepted observer source.

| Observation | Measured result |
| --- | --- |
| Maximum simultaneous browser + control-plane summed RSS | 1.10 GiB |
| Maximum complete observed driver-family summed RSS | 1.28 GiB |
| Maximum live owned processes | 13 |
| Worker cgroup memory peaks, in worker order | 44.48 / 44.93 / 45.36 / 44.76 MiB |
| Configured limit per worker | 1 GiB RAM, one CPU |
| Minimum whole-host available RAM | 15.63 GiB |
| Minimum root/workspace filesystem available space | 1.99 GiB |
| Sample interval median / p95 / maximum | 1.00 / 1.51 / 3.11 seconds |
| Final live owned-process count | 0 |

Process RSS sums overlap shared pages. Browser and API role peaks must not be added separately. Container cgroup memory is a different measurement; daemon and shared-kernel overhead are unassigned. Host RAM and filesystem availability include unrelated work. Root and workspace use the same filesystem, so their disk values must not be summed.

The observer began immediately after driver spawn, but there was no pre-run idle Luci baseline. Seven initial samples lacked the exact Docker ownership identifier. Some process I/O was unavailable in 21 samples. Counter deltas cover observed intervals only, and short-lived processes can be missed. Sampling included created or retained containers; running memory statistics were available in 12 / 12 / 12 / 14 samples respectively.

Source editing overlapped this run. A separately coordinated scoped compiler/test interval ran from 08:39:22.757618Z to 08:39:58.635189Z with GOMAXPROCS=2 and package parallelism one. That compiler was outside the observed driver family; its effects remain part of whole-host measurements. This is not a clean isolated performance benchmark.

The controlled provider made no external model calls. This result establishes installed execution and recovery mechanics at the recorded load. It does not establish live-model quality, UI input latency, native Windows behavior or whole-computer responsiveness. Its roughly 7.5-minute interval also does not complete the longer controlled/native soak requirements.

Use the [tracked observer instructions](../../../scripts/acceptance/four-project-queue/RESOURCE-MEASUREMENT.md) to reproduce observations. The earlier [e81 partial receipt](2026-10-08-e81b5860-partial-owned-resources.md) remains a failed, manually stopped two-result attempt and is not replaced by this result.
