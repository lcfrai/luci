# Sustained attempt 5: retained failure and bounded offline analysis

This analyzes the completed, failed installed attempt with product `2da1a821b8456c20a858fb86ef87649ea917d9de` and helper `e2f426bb50267a7cceac1f53eb1e00a0a4cc278e`. It is not another app run, a performance pass, a Windows result or a live-model quality claim. The original result SHA256 is `16f8f311d9cb0aca7a8d8b38c3be7eb3e1a8b4806e4b4c9247d8315dccc20ff9`.

## Workflow outcome

Two actual task receipts, Build A and Build B, were accepted. Both native sessions emitted exit-status 0 and later reached exact container absence, queue completion and disappearance of their own reservation. Each records one coding dispatch and one final response with no final response retry. These are two natural completions, not zero completions. Build A was integrated once through review. Build B was not integrated. Full four-project completion, the reviewed amendment and the settled idle phase were not reached.

The recorded failed stage is `actual-work-build-b`. The provider handler reports `Controller verifies actual task bundle did not complete within its bounded prerequisite window`. A third container was Created/Started for Build D while the driver awaited the anticipated next task's prerequisite. There is no accepted D command receipt. LOCAL-021 / CLOUD-043 track investigation/correction of that helper ordering assumption; the helper hardcodes an expected project order. This does not establish why the production scheduler selected D or demonstrate a production scheduler defect.

CLOUD-041 has a scoped installed positive: two compact command receipts reached the scripted provider and were accepted. The exact receipt fields, output bindings and native exits are retained; this positive does not close the overall run. Provider behavior is scripted, with 56 recorded provider requests and zero external model calls.

## Latency, with all observations retained

All 1,092 recorded actions succeeded: 400 idle and 692 partial-loaded actions. Every measured row, including draft resets and tails, remains in the projection. No duration, action or threshold was removed; no instrumentation cost was subtracted.

| Cohort | Switches / inputs / resets | Switch p95 | Input p95 | Switch p99 |
| --- | ---: | ---: | ---: | ---: |
| Idle | 100 / 200 / 100 | 326.012745 ms | 99.275165 ms | 387.560174 ms |
| Partial loaded | 173 / 346 / 173 | 626.344344 ms | 85.141110 ms | 1847.985361 ms |

Both switching p95 values fail the unchanged 300 ms target. Input p95 values are below the unchanged 100 ms target, but loaded coverage is below the required 200 switches / 400 inputs. Loaded switching p99 also exceeds the strict 1,000 ms bound; draft-reset p99 is 1000.039766 ms. This is neither a full latency gate nor a sustained-performance pass. Durations run from automation command through expected DOM content and two animation frames, and are not physical input-to-display measurements.

Eight idle and 78 loaded switches exceeded 300 ms. The larger loaded tail occurs in more than one stage:

- Original row 856 took 2218.056970 ms. The All projects click call took 1976.221137 ms, while its matched browser click handler took 44.5 ms and thresholded EventTiming duration was 128 ms. That identifies a large interval outside the synchronous handler, without identifying its cause.
- Row 784 took 1847.985361 ms. Its destination verification took 1657.243687 ms; the browser's Open-pointer-to-expected-DOM interval was 1662.4 ms. Its Open click handler took 5.6 ms. This is a different delay than the row 856 All projects call.
- Row 704 took 1376.737904 ms, including 1181.250168 ms in the All projects call, with a matched 37.5 ms click handler and 112 ms EventTiming duration.

No primary actionability trace or React CPU profile was recorded for these actions. Node stages include transport, actionability, scheduling and response waits; thresholded browser events cannot uniquely separate them. No specific component, GC or host scheduling cause is established, and these observations do not justify removing waits or subtracting overhead. The matched profiling diagnostic is a separate run and must retain its own scope.

## Sampled resources

The complete observer stream has 923 samples: 897 observed control-plane cgroup rows, 25 not observed and one partial. All 897 valid effective-memory formulas recompute without mismatch. Minimum observed effective available memory is 10,006,720,512 bytes; minimum physical available memory is 21,323,038,720 bytes. These have different scopes and are not atomic Create/Start admission snapshots.

The sampled owned-process RSS sum peaks at 1,397,080,064 bytes. Browser-role peak is 1,058,123,776 bytes and control-plane peak is 175,411,200 bytes. Role peaks occur at potentially different times, shared pages can be counted more than once, and these sums must not be blindly added to container cgroup memory. Container identity/usage coverage is A 255/225 samples (peak 455,933,952 bytes), B 294/289 (683,999,232 bytes), and the third container 123/122 (43,225,088 bytes).

The third container is visible in the owned resource sampler, transport records and independent closure, but is absent from the two `taskChildren` records. There is no equivalent driver-accepted D bundle/command inspection. Resource visibility is not completed task validation.

Observer interpreter self-cost is approximately 0.202021 core over 938.163438 seconds, excluding spawned Docker CLI/helper processes and all Docker daemon work. No matched no-observer causal estimate is available. The run is incomplete and lacks settled idle/growth/complete throughput evidence; no whole-computer responsiveness or Windows claim follows from these numbers.

## Closure and retained capacity

The independent read-only closure at 2026-10-09T02:51:54.362140Z reports 16 known PID/start identities: 14 absent and two Chromium zombies adopted by PID 1. Three unrelated process environments were unreadable. Docker container and volume inventories are empty. This is not whole-machine zero-process evidence.

The exact Build D reservation `queue-e271747355890121c64156ef`, project `p-f686c006`, session `s-58d19866`, remains `backend-settled` and charged 1,024 MiB / 1,000 mCPU. Ledger SHA256 is `336aee103bd74781fef244d4df7c519c0ccf3d8230614aa1b2e96f93a9d19079`. Empty Docker inventory does not prove that reservation was released. The driver records an unexpected remaining full worker and exact failed-fixture cleanup. LOCAL-016 remains the linked recovery/capacity obligation; no new root cause or automatic cleanup pass is inferred.

Root's closure records 629 fixture files and original artifacts unchanged. This publication does not repair, reconcile, replay or delete the original fixture or ledger. Private inventory bytes remain external; their unchanged status is the recorded closure observation rather than an independent public-byte retrieval claim.
