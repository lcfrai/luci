# Actual shared-cgroup observation beside the d20 idle run

This capsule preserves a bounded external observer run beside product `d20ebd292f3f1de04d6726bab0998c210987bcb5`, helper `c625ef0b7842748f4ce74cd3ffc08979d571bddf`. It establishes **positive shared-cgroup CPU bandwidth-throttling counter increments** during this observation. It does not identify the affected process or explain an individual slow project switch.

The [matching idle capsule](../cloud032-idle-d20ebd29-20261009/README.md) owns the exact integration wrapper, installed package/source bindings, all 432 action rows, and app closure scope. That run's switch p95 **424.671523 ms fails the unchanged 300 ms target**; input p95 **73.295114 ms passes 100 ms**. This adjunct neither subtracts overhead nor upgrades CLOUD-032 or the full sustained/Windows/live-provider acceptance gates.

## Retained run and counters

`runtime/cpu.original.jsonl.gz` decompresses byte-for-byte to the original 1,658,626-byte JSONL: one header, 1,728 samples, one footer. Every retained ordered counter object reconstructs its original `cpu.stat` raw hash and length. All adjacent deltas, previous-read bindings, fixed 100 ms schedule, limits and footer counts were checked independently. The source hard caps remain 400 seconds, 4,000 total rows and 2 MiB; no catch-up reads are used. No sample was unknown, reset, missed-deadline or overrun-labelled in this run. There were 1,740 fixed-file reads including both metadata groups; maximum read duration was 27.284875 ms and maximum lateness 37.899596 ms.

First-to-last counters increased by **13 `nr_throttled` and 125,756 `throttled_usec`**. Fourteen adjacent sample intervals contain a positive throttle field; these are not fourteen distinct events because counter fields can become visible in different samples. Shared `usage_usec` increased 225,872,091; this is not Luci-owned CPU time. No scheduler/profile/process discovery was added.

Before/after records have the same observed held cgroup inode/device, `cpu.max=400000 100000`, effective cpuset `0-4`, membership `0::/` and cgroup namespace. **`metadataStable=false` remains unchanged**: `/proc/self/ns/time` was unavailable with ENOENT both times. Equality of available records does not turn that field into an observation. The same held cgroup FD supports the positive counter arithmetic, but the record does not establish complete namespace identity, hidden-ancestor stability, cross-process clock calibration or no throttling elsewhere.

## Coverage and cost limits

The sampler recorded monotonic and wall brackets for each read. Exact phase labels copied from the private idle result are published in `analysis/phase-windows.json`; they lie within the recorded wall observation window. Coarse wall-labelled grouping gives quiet 2 positive intervals / +2 periods / +28,668 microseconds; primary interaction 9 / +8 / +90,721; follow-up zero; outside or across labels 3 / +3 / +6,367. These are nominal phase associations, not exact action attribution: no Node/browser/kernel clock-origin calibration or wall uncertainty bound was added. Counter increments cannot be assigned to the 15 slow switches, and the shared cgroup can include unrelated work.

The collector body used 1.373695062 CPU seconds through footer preparation; total observer-process CPU at that point was 1.474588725 seconds and **already includes the body**. The separate supervisor watchdog thread recorded 2.911927722 CPU seconds over 3,410 waits. These values exclude final serialization/fsync and other stated owner/startup costs, so they are not a complete adjunct-cost measurement and are never subtracted from latency.

The supervisor pinned only its own direct child with a pidfd. One accepted TERM after the workload ended yielded observer exit 0, a complete bounded serialization and normal exact-child settlement; no force was used. Its `observationComplete=true` refers to serialization/source/child checks, not complete usable metadata or product acceptance. `start()` pins and releases the bootstrap but does not promise a collector header before the unchanged workload starts; header PID and coarse temporal coverage were checked afterward. The adjunct performs no live app/browser census. Main-capsule driver/controller closure does not establish browser or whole-machine absence.

## Source, fixture history and reproduction

`prepared/` is an exact copy of the **earlier preparation** manifest and its 17 declared members, plus manifest/checksums. Its historical README and manifest correctly say not yet executed at that preparation time; the separate actual run receipts above are later evidence. Core fixtures passed 20 tests; final supervisor fixtures passed 31. Earlier 26/27-pass logs overlap; the intermediate 31-test fixture failure caused by an umask assumption is retained, followed by its corrected explicit-mode fixture. Counts are not additive across versions. The final cross-review is included. This capsule did not rerun those tests or start another workload.

To validate retained arithmetic without sampling or starting any app, use a fresh output path:

```sh
python3 analyze-counter.py --cpu runtime/cpu.original.jsonl.gz --phases analysis/phase-windows.json --supervision runtime/supervision-result.original.json --report /tmp/luci-counter-reanalysis.json
```

The analyzer accepts gzip directly; its report records the input argument paths, so a reproduced report at different paths is not byte-identical to the original report. Numeric observations and source bytes are retained, not regenerated. Prepared controlled fixtures can be inspected/reproduced with `python3 -m unittest -v test_observer` and `python3 -m unittest -v test_supervisor` from `prepared/`; the actual Linux-only integration wrapper is linked once in `linked-inputs.json`. Do not run an exclusive acceptance workload on a shared personal Docker environment merely to reproduce this offline audit.

`original-bindings.json` maps exact original/copy/gzip hashes and lengths. No CPU rows were redacted. Standard task paths and namespace IDs remain in these process-local observation receipts; no credential content, app state, binary or private full idle result is included. `analysis/independent-audit.json` is the independent read-only arithmetic/source/receipt audit, not a second runtime capture. `publication.json` binds all capsule members and the companion; `SHA256SUMS` also binds the manifest.
