# Installed Linux resource observation: partial execution

The e81b5860 installed attempt completed two real controlled Docker/Codex task outputs, reviewed integrations, a live-child service restart and cancellation of waiting work. The fixture explicitly stopped the first two workers after inspecting their outputs; natural worker completion and automatic capacity release were not verified. The workflow then failed because its transport evidence buffer dropped observations while a third worker started. This is **partial execution evidence**, not a completed four-project run.

The external observer retained 173 samples from 2026-10-08T06:51:34.178334+00:00 to 2026-10-08T06:54:36.635954+00:00. Sampling began after the test driver spawned and includes cleanup; no pre-run idle baseline was observed. Four projects were prepared, with a single worker admitted at a time. This does not test four concurrent workers or a sustained realistic software build.

| Measured quantity | Observed value |
| --- | --- |
| Maximum simultaneously live owned processes | 12 |
| Peak simultaneous browser + control-plane summed RSS | 1.07 GiB |
| Peak summed RSS including test driver and other observed descendants | 1.23 GiB |
| Browser peak summed RSS | 875.3 MiB |
| Control-plane peak summed RSS | 225.0 MiB |
| Test-driver peak RSS | 248.1 MiB |
| Lowest observed whole-host available physical memory | 18.58 GiB |
| Observed process CPU counter deltas, including test driver | 149.90 CPU-seconds |
| Whole-host CPU utilization p50 / p95 | 33.5% / 68.4% |
| Sample interval p50 / p95 / maximum | 1.00 / 1.53 / 5.06 seconds |
| Live owned processes after settle interval | 0 |

Separate worker measurements:

| Owned container | Observed samples / memory samples | Peak cgroup usage | Peak working-set estimate | Hard limit |
| --- | --- | --- | --- | --- |
| worker-1 | 10 / 7 | 54.9 MiB | 51.7 MiB | 1 GiB / 1 CPU |
| worker-2 | 8 / 6 | 53.8 MiB | 50.7 MiB | 1 GiB / 1 CPU |
| worker-3 | 4 / 1 | 25.8 MiB | 25.0 MiB | 1 GiB / 1 CPU |

RSS sums can count the same shared pages more than once; individual role peaks occur at different times and must not be added. Docker cgroup usage is deliberately separate. The Docker daemon, image cloning, shared kernel and unrelated host workloads are not attributed to Luci's process family. Host figures include unrelated activity and do not establish whole-computer responsiveness.

The first 39 samples lacked the exact worker-instance identity; process measurements continued and Docker ownership was explicitly unavailable. Later observations identified three exact-owned container IDs, with 14 samples of running-container memory in total. The third worker had only one memory sample. Sampling targets one second but scheduler and API delays produced gaps up to 5.06 seconds; short peaks and short-lived processes may be missed. Process I/O was unavailable for some tracked processes in 22 samples. These are observed values, not upper bounds or complete lifetime accounting.

The provider intentionally emits controlled commands. These small tasks do not establish real-model quality, heavier build capacity, speed superiority, UI input latency, native Windows compatibility or responsiveness of another app. Required native Windows and live-provider gates remain open. The [sanitized receipt and timeline](2026-10-08-e81b5860-partial-owned-resources.json) travel with the clone. They omit process/container identities and private paths while retaining numerical observations, missing-data flags and hashes of the private original evidence. The [reusable sampler and commands](../../../scripts/acceptance/four-project-queue/RESOURCE-MEASUREMENT.md) explain how to collect a new run. Neither command lines nor credentials are published.
