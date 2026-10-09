# 2da installed idle diagnostic: switch target remains failed

The exact packaged product and helper checkout were `2da1a821b8456c20a858fb86ef87649ea917d9de`. All 400 primary actions succeeded, but the original project-switch latency requirement remains **failed**. The result's `passed: true` records completion of the diagnostic procedure, not a passed latency or sustained acceptance gate.

| Primary action | Success / total | p95 ms | p99 ms | Unchanged target | Outcome |
| --- | ---: | ---: | ---: | --- | --- |
| All projects → open saved Define context | 100 / 100 | 369.714279 | 493.270531 | p95 ≤ 300 ms; p99 < 1000 ms | Fail |
| Idea input | 200 / 200 | 65.910818 | 109.987032 | p95 ≤ 100 ms; p99 < 1000 ms | Pass |
| Draft reset | 100 / 100 | 368.096866 | 484.971024 | No separate target | Functional success |

Every original numeric row is retained. Eight later trace rounds contribute another 32 actions and remain separately labeled; they do not replace any primary observation. `quantiles.recomputed.json` exactly reproduces all six original phase/kind summaries with nearest-rank quantiles. There is no filtering or instrumentation-cost subtraction. `python3 recompute-quantiles.py` verifies these public numeric rows offline without launching Luci, Docker or a browser.

The metric runs from the automation command through expected DOM content at two animation frames, including transport and any unsuccessful or timed-out duration. It is not physical input-to-display latency. This fresh-state installed application run uses the same frozen driver, collector, 60-second quiet period and 100/200/100 primary action counts as the preceding da8 diagnostic. Preparation/workflow/cleanup budgets remain 60/210/120 seconds. No independent Linux resource sampler ran. Unconditional guards reject Create and Start; the receipt records zero tasks, provider calls, auxiliary calls and external model calls.

## Explicit proxy difference

`proxy-followup-scope.original.json` is the exact recorded followup scope. The proxy source is `scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs` at SHA-256 `2b6887af0bad7975c74bac42c8ed7988813e3c72ba615281c727c0ac268b1606`. It incorporates CLOUD-040/LOCAL-019 known-check SDK stats recognition and validated query receipt changes. The idle driver's hard-zero Create/Start guards prevent check allocations. This does **not** establish that every helper is unchanged from the preceding run, that read-side proxy costs are identical, or that a latency difference is caused by the product correction. Frozen driver/workload/collector continuity and the changed proxy are separately bound.

## Identity and reproducibility

`binding.json` ties every original artifact hash to the result, closure, package and executed helpers. `accepted-source-bindings.json` and `sources/accepted/` retain all 16 exact accepted helper snapshots, gzip-compressed with original decoded hashes. Every accepted snapshot matched its prepared counterpart; all seven repository dependencies also match their immutable 2da Git blobs. `sources/template/` retains the exact original 16 source files. The preparation script and extraction/proxy patches are byte-for-byte copies; launcher and analysis script copies explicitly substitute private preparation paths. Their original hashes are retained. These substitutions are publication representations, not the executed bytes.

The frozen generated driver is `d7fb7586e5b4a2639da9abb6c2e1c6a1296d2161ea7cf2ec393a8f1d3ab1b016`; the interaction collector is `ec0bc9d06fda6636cbba6975e05b9c66979ca189b59aa38c373b37a318b79502`. A different external execution-directory binding is expected; it does not change the collector. `invocation.sanitized.json` preserves all exact switches, source/image pins, budgets and hashes with explicit filesystem placeholders. To reproduce, retrieve the recorded clean helper/product source, restore exact source snapshots and dependencies, provide the matching package and image provenance, prepare a fresh external diagnostic directory using the retained preparation script, and substitute only the documented locations in that invocation. The helper requires an exclusive disposable Docker environment; do not use it on a shared personal daemon or infer permission to touch other workloads. The preserved full sustained helper has additional prerequisites; this idle-only extraction does not run its workloads.

Package archive/binary/web hashes and the recorded 3,587 matching archive members bind the installed run to 2da. The image projection retains all 104 before/after source hashes from the original image receipt. The capsule preparation did not redownload/rebuild the package or image, inspect the daemon, or revalidate private archived bytes. Historical package/image verification remains the original publisher's receipt.

## Closure

The exact wrapper records exit 0, no external-monitor timeout and 155.917330 seconds elapsed (2026-10-09 01:38:05–01:40:41 UTC). The final result records completed controller/browser/provider/proxy cleanup, drained requests, certain mutation state, removed fixture, zero owned service/browser processes and zero containers/volumes. The byte-for-byte independent closure separately observes the recorded driver PID/start identity absent, clean 2da source, expected image identity and empty daemon inventories at 01:40:56 UTC. Full app/browser identity closure additionally relies on the driver receipt; this is not a whole-machine zero-process assertion.

## Analysis limits

This is one actual 2da arm. No new static-control arm was run. Existing numerical analyses are retained in `analysis/`, with original hashes. The raw trace was not reopened during capsule preparation. Temporal EventTiming correlation and the later trace followup do not identify a React function as the cause of primary tails. Different clocks and differently bounded components must not be added or subtracted to change the acceptance metric.

The preceding da8 p95 switch/input values were 317.287164/66.501146 ms. These sequential single-arm observations do not establish a causal product gain or regression; environmental variation and the proxy followup were not isolated. Source-test evidence for the product changes remains separate from this failed latency gate. Loaded four-project latency, sustained memory/CPU/fairness, Windows behavior and full local acceptance remain open.

## Privacy and representation

The original result SHA-256 is `b949714a6b68b1ef51fb2bae96b6af02fbd3a2158047b94ca2221ff7b6207637`. Raw result/trace and private runtime paths remain externally retained and are not included. `result.projection.json.gz` is an allowlisted projection, not an exact raw result: it preserves selected receipt fields and all 432 numeric rows while omitting project IDs, fixture contents, private paths, browser/network ledgers and raw event payloads. Its inherited generic scripted-provider limitation text does not imply worker or provider execution in this idle run. Numeric analysis artifacts are also derived from originals, even where their own bytes are copied exactly.

`privacy-scan.json` records recursive JSON-string and decoded gzip/text checks for the known run/preparation roots and credential-shaped values available in the original result. It is not a universal secret-scan guarantee. Generic repository and operating-system path literals in immutable source code are preserved. Only this capsule is intended for later publication, not the external execution directory. Preparation writes exclusively outside the checkout; it does not alter the source or run runtime workloads.
