# Installed idle diagnostic before Home card memoization

The original 400-action primary diagnostic completed successfully at clean source `aadc1c281aef28aee04046a5c5e58da5fe7094e4`. The original project-switch latency target remains **failed**; input meets its original target. This is a diagnostic subset, not full sustained acceptance and not validation of the later Home rendering patch.

| Primary interaction | Successful / total | p95 ms | p99 ms | Original target | Outcome |
| --- | ---: | ---: | ---: | --- | --- |
| All projects → open saved Define context | 100 / 100 | 595.146122 | 671.142952 | p95 ≤ 300 ms; p99 < 1000 ms | Fail |
| The idea input | 200 / 200 | 65.970813 | 239.900350 | p95 ≤ 100 ms; p99 < 1000 ms | Pass |
| Draft reset | 100 / 100 | 567.938082 | 720.060442 | No separate target declared | Functional success |

All primary rows are retained in the numeric projection; no row filtering, endpoint changes, or resource-cost subtraction was applied. Nearest-rank p50/p95/p99/max and success counts were recomputed from the original interaction durations and match its six summaries exactly. An additional 32-action trace followup is separately labeled and excluded from the primary targets. The raw trace is not included or reopened for this preparation.

The metric runs from the automation command through expected DOM content at two animation frames, including transport and any unsuccessful or timed-out action duration. It is not physical input-to-display latency. The fixed primary workload is 100 switches, 200 inputs and 100 resets after the original setup and 60-second quiet period. Preparation/workflow/cleanup budgets are 60/210/120 seconds. No Linux resource sampler was active. Unconditional guards deny Create and Start; zero tasks, provider calls, auxiliary calls, external model calls and cloud resources were recorded.

## Source and closure binding

`binding.json` records original artifact hashes, package/source/helper identities and offline consistency checks. `external-source-binding.sanitized.json` preserves every executed helper/dependency hash and the documented proxy dependency difference. The driver remains `d7fb7586e5b4a2639da9abb6c2e1c6a1296d2161ea7cf2ec393a8f1d3ab1b016`; interaction collection remains `ec0bc9d06fda6636cbba6975e05b9c66979ca189b59aa38c373b37a318b79502`. The external execution-directory binding changes with its location; it is not the fixed collector identity. Package archive/binary/web hashes and 3126 matching archive members bind this run to the installed aadc product. Current checkout files and package bytes were not revalidated during this evidence preparation; recorded run receipts establish those historical checks.

The exact wrapper receipt reports exit 0, no external monitor timeout and 159.089 seconds elapsed. The final result records completed controller/browser/provider/proxy cleanup, drained Docker requests, certain mutation state, zero containers/volumes/owned service and browser processes, unchanged protected image/package/helpers and a removed fixture. The exact independent closure receipt binds the result SHA, clean aadc source, image ID and recorded driver identity/absence; it independently checks daemon/source/known-driver state and relies on the final driver receipt for full app/browser identity closure.

## Privacy and representation

The source result SHA-256 is `61fe4f48f685af6c4a4402f14ee8cc40d3ee4d0cb052daf2eafdad59482fdc61` (7,327,214 original bytes). Its `externalSourceBinding.executionDirectory` contains a private path. Invocation and provenance artifacts also contain private paths. Their original bytes remain preserved externally; no original result gzip is included. `result.projection.json.gz` is a **sanitized allowlisted projection**, never an exact-raw result. It includes the selected receipt fields and all 432 numeric interaction rows, omitting raw trace/event/network data, request paths, project IDs, fixture content and private filesystem locations. Other sanitized files explicitly state their omissions and bind to their original hashes. Only the wrapper and independent closure JSON are byte-for-byte originals. Publication should include only this directory's files, never the private diagnostic directory.

## Interpretation and recommendation

The existing passive temporal correlation reports All-projects→Open browser elapsed p95 275.300 ms and Open→expected-DOM p95 124.300 ms; the All-projects click handler has p95 46.200 ms and maximum 181.200 ms. These are differently bounded observations, not additive cost components or React/function attribution. Its thresholded EventTiming data, sequential single arms and separate trace followup do not establish which source function causes primary tails. Prior d348 switch/input p95 values were 400.461/132.943 ms; this comparison is descriptive, not causal.

The staged Home correction memoizes project cards and stabilizes selection callbacks so an independent overview response does not rerender unrelated cards. This preserves fresh overview GETs, polling, abort cleanup, authority invalidation and selection/queue behavior. It targets repeated summary renders; it does not remove initial Home mount cost and cannot yet be claimed to fix the measured switch target or the 181.200 ms handler. Review and targeted regressions can validate rendering/authority behavior; a later authorized fresh installed run with the same frozen collector and original targets is needed for latency acceptance.
