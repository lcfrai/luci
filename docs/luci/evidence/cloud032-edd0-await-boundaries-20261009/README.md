# edd0 / 8c7 await-boundary diagnostic: retained analysis

The exact installed run completed in **174.193748 seconds**, wrapper exit 0. Product is `edd0cb121ff6847f681ab6bf41509a497f043d41`; helper is `8c7bf16aa035b27dbea185b565052495eb60777f`. Original result SHA256 is `b37392da16cdae01ea9096f9486148f7ac765412ec71e9d41f9dd507324fcf19`. The current checkout is deliberately not used as the run identity because subsequent work may change it.

All **432 original actions succeeded**, but the primary project-switch p95 **582.760 ms still fails 300 ms**. Primary input p95 **76.478 ms meets 100 ms**. Nothing is subtracted from the original full spans, and no new overall acceptance or gain relative to earlier products/runs is claimed.

| Cohort | Count | p95 | p99 | Maximum |
| --- | ---: | ---: | ---: | ---: |
| Primary switches | 100 | 582.760 ms | 666.562 ms | 673.358 ms |
| Primary inputs | 200 | 76.478 ms | 189.226 ms | 350.157 ms |
| Primary resets | 100 | 549.994 ms | 602.685 ms | 684.225 ms |
| Separate trace switches | 8 | 275.381 ms | 275.381 ms | 275.381 ms |
| Separate trace inputs | 16 | 63.228 ms | 63.228 ms | 63.228 ms |
| Separate trace resets | 8 | 627.335 ms | 627.335 ms | 627.335 ms |

Reset rows have no declared 300/100 ms target. The separate trace cohort does not replace the primary cohort or explain individual primary actions.

## Complete observations and stage findings

All 108 switches, including trace, contain exactly the **12 expected ordered Node marks**. All 324 inputs/resets contain the original six. No unknown, missing, reordered, duplicate or over-cap marks were accepted; every verification interval equals its three new components within numeric tolerance. The exporter preserves all 432 full rows, including success and full duration, and checks the exact per-phase action counts.

The observer retains **100 Home ready and 100 Define ready**, plus one closed lifecycle row: 100 complete primary switch joins, no missing/censored/ambiguous stage, no sequence gap, overflow or late callback. Define observed-ready to driver expected-DOM lag has p50 30.3 ms, p95 62.3 ms, p99 166.5 ms, maximum 195.1 ms. Home/Define each have two maximum-sample gaps above 50 ms and none above 100 ms. Sampling gaps still limit first-readiness interpretation; a structural predicate is not proof of continuous readiness, visibility, actionability or paint.

| New inclusive Node region | Primary p50 | Primary p95 | Maximum |
| --- | ---: | ---: | ---: |
| Open command | 82.578 ms | 124.180 ms | 364.124 ms |
| Textbox waitFor region | 32.917 ms | 52.015 ms | 77.143 ms |
| Saved-value Locator.evaluate region | 16.205 ms | 48.114 ms | 177.827 ms |
| inputValue/assertion region | 4.109 ms | 10.108 ms | 25.572 ms |

The new evidence narrows two specific tails. Original index 300 has a 473.431 ms full switch and 195.1 ms observed-ready-to-driver lag; its Node waitFor/evaluate/inputValue regions are **36.011 / 177.827 / 25.572 ms**. Index 352 has a 582.760 ms full switch and 166.5 ms observed-ready-to-driver lag; its regions are **42.858 / 117.729 / 24.647 ms**, alongside a 270.137 ms Open command. These gaps are predominantly within the evaluation command, not exclusively inside textbox waitFor or the final value read. The regions still include transport, lookup and scheduling; this does not establish app work.

Other tails have different locations. Index 344 is the largest full switch at 673.358 ms, with a **474.482 ms All-projects command**; Home is structurally observed 35.0 ms after its trusted click. Index 108 has a 411.690 ms All-projects command. Indices 72 and 268 have **248.403 / 257.148 ms heading waits**, while Home is observed 15.3 / 16.3 ms after the trusted click. Across the 25 switches exceeding 300 ms, the largest recorded Node region is All-projects in 11, Open in 11, heading wait in 2, and evaluation in 1. This is a descriptive classification, not an exclusive CPU or causal attribution. Percentiles must not be summed and browser-clock observations must not be subtracted from Node timestamps.

## Concrete next step; no product optimization established

The exact edd0 Definition loader performs one cancellable specification read and publishes the loaded value and ready state together; it does not await the Orchestrator read. The original evaluation callback tests a retained element's connected/value predicate and finishes immediately when that sample matches, otherwise polling by animation frame. It does not by itself identify React work.

The installed Playwright source inspected after the run adds a meaningful interpretation: **Locator.evaluate includes fresh strict attached-element lookup, evaluation through an ElementHandle, then awaited disposal**. Its inclusive Node span is not one pure browser callback. The existing frozen runtime capsule did not seal node_modules bytes, so this current dependency-source explanation must not be represented as proof of the historical executed dependency bytes. Exact paths and this boundary are recorded in `source-review.md`.

A bounded next diagnostic is a short matched control on the same settled real Define DOM: alternate the original Locator.evaluate route and a pre-acquired handle to the exact same editor, use the identical callback in both arms, and return callback-entry/first-sample/sample-count metadata through that callback's existing response. Keep original acceptance actions, spans and thresholds unchanged and separate. Refuse changed project/context/authority/document/element identity, missing/disabled/disconnected editor, and late callbacks; retain failures without retries. Pre-acquire handles outside measured regions and dispose them with bounded cleanup. This can estimate the contribution of lookup/disposal versus callback waiting under that settled condition; it cannot certify a universal transport floor, first readiness, paint or an app improvement. No new run or implementation of that control occurred in this analysis, and no speculative product correction is justified by these spans alone.

## Provenance, closure and privacy

`actual/` contains all original 432 allowlisted interaction rows, all 201 readiness rows and 102 drain windows, all 100 joins, exact boundary-label validation, all 432 joined stage rows, original quantiles, project/block groupings and immutable input bindings. There are zero analysis issues. `analyzer-binding.json` discloses the only changes from the prior analyzer: the expected frozen-helper directory and the three explicitly added stage labels. Threshold, quantile and join/censor logic are unchanged. The separate boundary exporter is the exact frozen candidate implementation.

All **20 accepted helper files** match the pre-run runtime capsule byte-for-byte. `runtime-candidate-frozen/` preserves that complete source set, source bindings, launcher, original marker candidate, its eight controlled tests and the four new inert integration tests with raw outputs. Those tests are historical preparation evidence, not rerun or counted as new analysis tests. `original-public-receipts/` retains exact wrapper, invocation, driver identity and package-provenance bytes. `completed-operations.json` records completed analysis commands and their observed summaries honestly; it does not invent raw subprocess logs where the tools returned stdout directly.

`independent-closure.json` records exact journal-bound driver **2407939** and controller **2408148**, both absent at observation. The **43 journal rows are contiguous**. The explicit recorded Unix daemon has zero containers and zero volumes, with image and engine identities matching the run. All bounded original run files, journal, trace and accepted sources are unchanged across closure. The identity reader distinguishes same-identity zombies, reused PIDs, unreadable identities and absence; it does not treat a zombie as absent.

**Independent historical browser identity closure is unavailable:** the retained run has no browser PID/startTicks list. Browser closure, fixture removal and zero-owned-browser counters remain labelled driver assertions. No broad process-environment scan, guessed browser attribution, whole-machine absence, signals, private-fixture inspection or cleanup mutation was performed. The current repository is not required clean or equal to the old helper after the run.

Private raw result/journal/trace payloads remain retained in place and are excluded from this capsule. Published rows map project IDs to owner-order ordinals and omit names, contexts, values, credentials, raw request URLs, free-form errors and private fixture paths. No CPU, overhead subtraction, physical display or causal performance claim is made.
