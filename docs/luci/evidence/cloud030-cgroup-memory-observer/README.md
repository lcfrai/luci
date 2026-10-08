# CLOUD-030: visible control-plane cgroup memory observations

The observer previously retained physical host `MemAvailable` and exact container measurements, but not the control-plane process's visible cgroup memory limits. This additive helper change records those limits, their current usage and selected `memory.stat` counters. It does not change product admission, workload limits, acceptance thresholds or any historical receipt, and it does not identify the cause of attempt 8's memory refusal.

The implementation stays inside `sample-linux-owned.py`, so the DAG and sustained wrappers' frozen standalone sampler copies need no extra imports. Existing raw host fields remain; `host.memory.physicalCountersValidated`, top-level `controlPlaneCgroupMemory`, and corresponding metadata are additive. The existing resource totals and container ownership logic are unchanged.

## Meaning of the new fields

Only already-owned, live descendants classified as control-plane candidates are considered. This existing classification uses the process name after descendant/PID/start-time ownership checks; it does not authenticate executable bytes. Each observation rechecks PID/start identity, both namespaces, membership and mount topology. Different observer/target cgroup or mount namespaces, missing access, PID reuse and topology movement make the estimate unavailable. Raw membership and mount paths are retained only as SHA-256 hashes.

`productComparableEffectiveTotalBytes` and `productComparableEffectiveAvailableBytes` follow the pinned [product probe sources](product-basis/): independent minima of validated physical total and every finite visible limit; and physical available versus `max(0, limit-current)` at each visible ancestor. The leaf and every ancestor through the visible mount root are considered. Missing limit files remain explicitly **missing**, and are skipped only to match product semantics. V2 `max` is unlimited at that node; v1 numeric sentinel values retain the product's finite-numeric interpretation. Missing or invalid usage for a finite limit invalidates both effective estimates. No memory-stat, RSS, swap, soft-limit or scheduling-reservation subtraction enters this formula.

`memory.stat` contains allowlisted numeric counters, separately marked observed/unavailable. Those counters are diagnostic only. Hierarchy usage is shared and must not be summed across levels or duplicated processes. The estimate is sequential and sampled, not atomic, not the product's own contemporaneous decision, and not evidence about invisible ancestors or an earlier refusal.

Observation limits are eight targets, 128 visible levels, 128 emitted nodes, 512 read operations, 1 MiB bounded file bytes and 128 KiB serialized output per call. The 50 ms time budget is **cooperative between synchronous reads**; it cannot interrupt a blocked kernel/filesystem read. A limit hit is explicit and does not substitute raw physical availability for an unavailable estimate. No credential, environment, command-line, provider or workload payload is read.

## Retained checks and scope

- [controlled2](controlled2/receipt.json): 48 passing controlled tests, zero failures, including 20 unchanged sampler tests and 28 added hierarchy, parser, identity, namespace, numeric, bounds and actual-main capture cases. All inputs are fixtures; these tests do not query native cgroups or Docker.
- [controlled1](controlled1/receipt.json): the initial 44-test result remains preserved. Read-only review subsequently identified unsigned-sign parsing, huge diagnostic counter conversion, strict physical units and CPU-only ancestry edge cases. The corrected cases are included in the final 48-test result; [review-scope.json](review-scope.json) states the independent review scope.
- [native2](native2/result.json): three native read-only observations using the final sampler bytes. The target was the tiny Python probe itself, explicitly supplied as a role candidate, **not an actual Luci process**. All three returned observed/stable results. The first recorded a 32 GiB visible cgroup limit, effective available 14,748,594,176 bytes, and physical available 21,534,863,360 bytes. This is new diagnostic evidence, not a reconstruction of attempt 8.
- [native1](native1/result.json): three earlier observations and their exact earlier source remain separate. No receipt was overwritten.

The final native calls took approximately 1.84, 0.97 and 46.83 ms, including scheduling delay. The native interpreter's lifetime `RUSAGE_SELF` was approximately 0.405 CPU seconds and 41.95 MiB peak RSS, including Python imports and the capture script; this is not incremental per-call overhead. During installed runs the wrapper's interpreter self-cost includes these new sampler reads. Shared cgroup usage may include observer work; neither self-cost nor process RSS is subtracted from cgroup current usage. No performance acceptance or zero-overhead claim follows.

No app, browser, Docker, cloud allocation, model or compiler was started for these checks. The tiny native captures were explicitly authorized during the root's broader attempt 9 work window; their UTC timestamps are retained for comparison with that run, not as isolated performance measurements. This capsule establishes neither that attempt's result nor a sustained-run result.

## Reproduction

The exact final standalone sampler, new tests, unchanged tests and native capture script are under `source/`. Use a fresh external directory; do not overwrite the retained native captures.

```sh
set -eu
C030_EVIDENCE=/workspace/luci/docs/luci/evidence/cloud030-cgroup-memory-observer
C030_REPRO=/workspace/.luci-tools/cloud030-reproduction
test ! -e "$C030_REPRO"
mkdir -p "$C030_REPRO/staged" "$C030_REPRO/evidence"
cp "$C030_EVIDENCE/source/sample-linux-owned.py" \
   "$C030_EVIDENCE/source/sample_linux_owned_test.py" \
   "$C030_EVIDENCE/source/cgroup_memory_observer_test.py" "$C030_REPRO/staged/"
cp "$C030_EVIDENCE/source/native_readonly_capture.py" "$C030_REPRO/"
cd "$C030_REPRO/staged"
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest -v sample_linux_owned_test.py cgroup_memory_observer_test.py
# Run native observation only in the root-coordinated slot:
PYTHONDONTWRITEBYTECODE=1 python3 "$C030_REPRO/native_readonly_capture.py" "$C030_REPRO/evidence/native-reproduction"
```

`manifest.json` binds the exact final source and retained receipts. `SHA256SUMS` covers the capsule. The product-basis hashes are also embedded in sampler metadata, so this calculation's reviewed source basis remains explicit when the helper and packaged product use separate commits.
