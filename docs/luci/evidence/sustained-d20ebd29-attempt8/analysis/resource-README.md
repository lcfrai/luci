# Sustained d20 attempt 8: retained resource observations

This read-only analysis uses the original observer artifacts from `<PRIVATE_RUN_EVIDENCE_DIRECTORY>`. It does not execute an app, provider, benchmark or fixture. Product `d20ebd292f3f1de04d6726bab0998c210987bcb5` and helper `5f318d5178210e95cd2b773f07686d88936d603e` identify the overall run; workflow/source/latency bindings are owned by the companion full-run capsule. Its private original driver result SHA256 is `c9a6ec78461a982f9e0f09a5e043601a7bd96cbd3a6ebe91c11e9034de93df4e`. This resource analysis does not reconstruct that private result.

All **1,573 samples** are included. The raw 10,260,910-byte stream has SHA256 `f3ec77d61caf7afa4dc22e5eea9695559457d709ccc6445419a0d5c872c34287`; it is published once by the main capsule, not duplicated here. Samples span `2026-10-09T10:12:45.264626+00:00` through `10:39:06.431342+00:00`, or 1,581.166716 seconds between first and last timestamps. There is no pre-run idle resource baseline.

The control-plane memory observer produced **1,548 observed samples/observations and 25 not-observed samples**, with zero formula discrepancies or unverifiable observed formulas. For each observed row the analyzer independently takes the minimum of physical available memory and each visible finite ancestor's `max(0, limit-current)`; total memory takes the independent minimum of physical total and those finite limits. It uses neither cache discounts nor execution-reservation subtraction. Minimum effective available memory was **5,828,194,304 bytes**, compared with minimum physical available memory **17,314,258,944 bytes**. These minima occur over the sample series and are not asserted as simultaneous admission operands.

Peak sum of owned-process RSS was **1,508,913,152 bytes**; role peaks were test driver 690,012,160, browser 1,030,676,480, control plane 174,456,832 and other owned descendant 14,884,864 bytes. Role peaks need not occur together. RSS double-counts shared pages and includes the acceptance driver/browser. Container cgroup usage is a separate measure and must not be added to process RSS as unique RAM.

Observed process CPU deltas total **1,178.95 seconds**: test driver 200.01, control plane 121.68 and browser 857.26. These cover exact PID/start identities only between their observations, not before first observation, after last observation or unseen short-lived children. Observed physical I/O counters total 4,501,504 read bytes and 25,939,968 write bytes; logical `rchar`/`wchar` counters are separate and are retained in the analysis. Docker daemon/shared-kernel work remains unassigned.

Five exact containers have sampled identity/usage coverage:

| Sandbox | Identity rows | Usage rows | Peak observed cgroup memory bytes |
| --- | ---: | ---: | ---: |
| `s-1aea8216` | 247 | 219 | 447,365,120 |
| `s-d5be84fc` | 319 | 297 | 698,826,752 |
| `s-e41c7f40` | 238 | 226 | 422,690,816 |
| `s-a367b1ab` | 231 | 228 | 419,819,520 |
| `s-06a08cb8` | 53 | 40 | 332,632,064 |

All immutable container IDs, process identities, observation timestamps and coverage gaps remain in the analysis and original rows. Sample coverage alone does not prove execution, natural completion, integration or result quality. Check-allocation observation was not enabled, so no check resource evidence is claimed.

All host CPU/memory/I/O pressure observations are **unavailable**, not zero. Forty-three sample intervals exceed 1.5 times the nominal one-second interval; their original brackets are retained. Maximum interval is 3.292614165 seconds; whole-sampler duration p95/max is 0.506535375/2.452006635 seconds. The memory-observation portion p95/max is 0.002293017/0.049573015 seconds; its 50 ms budget is cooperative between synchronous reads. No gaps were filled or samples dropped. The process scan recorded 237 unreadable/raced entries and four initial exact-instance-not-yet-provided observations; neither is a silent success.

The sampled disk minima are `/` and `/workspace` 1,431,289,856 bytes, `/tmp` 2,123,026,432 bytes. Paths may share a filesystem and must not be summed. Sampled host CPU reached 79.381443299 percent between adjacent observations, but neither this value nor disk/memory values reproduce any product admission decision or explain an earlier refusal.

Observer `RUSAGE_SELF` reports 366.628024 CPU seconds over 1,581.536109 seconds, or **0.231817675 core**; maximum observer RSS is 20,475,904 bytes. This includes interpreter/import/launcher work but excludes spawned Docker CLI/helpers and daemon work. It is not total observation overhead and is never subtracted from application measurements. This run did not use the earlier idle run's separate shared-cgroup CPU-counter adjunct; its throttling observations must not be backfilled into this run.

The sampler's 1,556 `post-start-running` and 17 `post-driver-exit-settle` rows are observer lifecycle phases, not the workflow's idle/loaded/settled cohorts. Seventeen zero-RSS post-driver rows do not establish complete cleanup. Two Chromium identities were last observed as zombies; the root's independent closure separately reports 13 absent known identities, two exited Chromium PID-1 zombies, three unreadable unrelated environments and empty owned Docker/volume/ledger inventories. This resource analysis does not itself certify those live closure observations or whole-machine absence.

`resource-analyze.py` is adapted from the frozen sustained7 analyzer and reads one bounded raw sample at a time, plus bounded numeric aggregates. It accepts either original JSONL or the exact gzip representation used in the main capsule. To recompute offline into a fresh output directory:

```sh
python3 resource-analyze.py --base /path/to/full-run-capsule --out /tmp/luci-sustained8-resource-reanalysis
```

The base must contain `observer/{metadata.json,summary.json,accepted-sampler.py,samples.jsonl.gz}` and `observer-observer-self-cost.json`. The report records current generation time and analyzer ancestry availability, so different locations need not yield a byte-identical report; the input hashes and arithmetic must match. No workload is launched by this command. Original rows, summary, unavailable state and observer self-cost remain independently bound.

The full run's scripted functional positives and responsiveness failures belong to its workflow/latency evidence. This report is not an atomic admission replay, causal improvement claim, native Windows result, live-model quality check, complete resource budget pass or whole-computer responsiveness acceptance.
