# CLOUD-032: installed idle diagnostic at d20ebd29

**Project switching fails the primary p95 target; input passes. CLOUD-032 remains open.** Installed product/package was `d20ebd292f3f1de04d6726bab0998c210987bcb5` and the clean helper checkout was `c625ef0b7842748f4ce74cd3ffc08979d571bddf`. The fresh diagnostic completed with wrapper exit 0 in 171.580 seconds and all 400 primary actions successful. `passed=true` and `idleDiagnosticCompleted` mean bounded diagnostic capture completed, not that timing targets or the local product passed.

| Primary cohort | Count | Median | p95 | p99 | Target |
| --- | ---: | ---: | ---: | ---: | --- |
| Project switch | 100 | 236.001259 ms | **424.671523 ms** | 704.605302 ms | p95 ≤ 300 ms; failed |
| Input | 200 | 49.812317 ms | **73.295114 ms** | 123.009714 ms | p95 ≤ 100 ms; passed |
| Draft reset | 100 | 268.291748 ms | 582.770518 ms | 685.571610 ms | No declared p95 target; separate cohort |

The unchanged switching/input p99 requirement is strictly below 1000 ms. The separately traced eight-round follow-up contains 32 successful actions; its switch/input p95 is 303.576185/79.504404 ms. Those smaller, differently instrumented cohorts do not upgrade the primary result. All 432 actions and numerical spans are retained without filtering or instrumentation subtraction.

## What the evidence supports

Fifteen of the 100 primary switches exceeded 300 ms. `analysis.json` retains every slow action's original Node spans, browser pointer/driver-expected-DOM/frame references and temporally overlapping resource categories. Node spans include automation transport, actionability and scheduling; individual p95 values cannot be added. Expected-DOM is sampled by driver evaluation after a locator wait, so these are **not autonomous React commit, physical display or isolated backend timings**. Resource overlap does not identify an exact project's request or a causal function.

The separate follow-up trace records 40 click calls, each with one click attempt, one visible/enabled/stable witness and one completed click. It does not cover the original 15 slow primary switches or establish their cause. No CPU profile weighting or causal improvement from CLOUD-048/050/051/052, response cancellation, layout stability or any other change is claimed. Earlier response-header intervention diagnostics remain separate; this run used the unchanged idle action protocol.

This run allocated no workers: zero task results, provider calls and external-model calls, hard zero-Create/Start guards and fresh state. Docker readiness/inventory reads still occur. A separate [same-cgroup counter adjunct](../cloud032-cgroup-d20ebd29-20261009/README.md) ran around the launcher. Its own publication owns counter rows, observer/supervisor source, supervision receipts and phase analysis; none is duplicated here. It is not an owned-process memory sampler, atomic admission snapshot, causal per-action attribution, sustained execution, model-quality, Windows or whole-computer responsiveness result. No previous run's RAM/CPU figures or zombie counts are transplanted to this one.

## Source, transformations and reproduction

- `timings.projection.json.gz` retains all **432 original action rows** and **3056 original browser latency rows**. Only each interaction's project ID is removed; numerical diagnostics remain exact. The original private result is 7,499,346 bytes, SHA256 `da1cd7eb46948775af3d5641389940776a4c75f34c1bc71b42bef9ce42c79a77`. A projection is not a reconstruction of all original fields.
- Original analysis and trace projection are copied unchanged alongside their exact derivation helper. Trace projection omits selectors, parameters and body values; the full private trace remains hash-bound, not published.
- `source-bindings.json` binds **17 accepted helper members**, including **seven imported repository dependencies**, and **17 overlapping full-template inputs** against exact product/helper Git objects. Image receipt projections bind **104 unchanged runtime source inputs** to the product commit. The original image and binary/archive bytes remain publisher-held; publication did not rebuild or directly audit them.
- The executed idle driver is byte-identical to the preceding 990 idle driver. C049 changes only the full sustained phone tail, removed by the existing idle extraction. Captured input/driver equality preserves protocol continuity; it is not a controlled causal speed comparison between product revisions or hosts.
- `sources/preparation/` retains the exact generator, launcher, extraction patch, source relationship proof and preparation README. The archived 67a5 preparation clean-tree assertion initially failed on concurrent documentation writes; its original script and labelled failure summary are retained. No app was launched by that preparation failure, and unavailable original per-subprocess syntax logs are not invented. That historical preparation event is not a new d20 failure. The reused runtime launcher still required exact clean helper source. This is distinct from the earlier 990 closure helper error, which did **not** recur here.
- `sources/run-idle-d20-with-counter.py` is the exact outer wrapper for this run. It launches the unchanged idle preparation and manages the separate adjunct; the companion capsule binds its outer invocation/result and supervisor identity. Reproduction needs both source capsules and the exact package/image, with task-specific paths adapted to fresh locations.
- Standard provenance paths remain in copied helpers/receipts; projected metadata replaces external execution, extracted package, archive and private fixture-parent locations. Original hashes/lengths remain bound. No private application/credential/retained failed-fixture files were read by publication.

Use the archived preparation README to reproduce in a fresh **exclusive empty Linux Docker environment** with the exact source, package and image. Paths are task-specific; do not run this exclusive harness against the Windows tester's shared Docker daemon or over retained evidence. The product is retrievable from the source commit; archive bytes remain private cloud artifacts. This capsule does not change setup requirements or authorize starting Windows workloads below the local tester's existing memory gates. `analyze.py` derives projections from private originals; it was already run by root and was not rerun for publication.

## Closure and limits

The wrapper retains `cleanupUnconfirmedByWrapper=true`: wrapper exit alone cannot certify cleanup. The final driver receipt reports all five startup handles closed, controller/proxy settlement, empty cleanup errors and fixture removal. Root's independent **read-only** receipt confirms only the exact recorded driver and controller PID/start identities are absent, Docker containers/volumes empty on the same exclusive engine, the same image ID and unchanged original artifact hashes.

There was **no independent browser PID/start census**. Browser/zombie absence and whole-machine closure are not established; historical PID-1 zombies from the prior sustained run remain explicit in that run's evidence and are neither assigned to nor cleared by this diagnostic. Fixture removal is the driver's assertion, and its absent final private path prevents independent fixture-path verification. No old failed fixture or execution ledger was read or edited.

`publication.json`, `SHA256SUMS` and `original-bindings.json` bind the published files and exact original copies. The companion report is bound separately. All acceptance gates remain open at their existing scopes; this failed timing result is not a `LOCAL TEST HANDOFF READY` declaration.
