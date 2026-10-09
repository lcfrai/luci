# d20 autonomous-readiness attempt 1: retained analysis

Product `d20ebd292f3f1de04d6726bab0998c210987bcb5`, clean helper `f43542a84bd9cd81c2b263e680c36d8225e10024`, frozen external observer v1. The actual wrapper completed in 174.106 seconds with exit 0. All 432 original actions succeeded. Primary project-switch p95 **591.025 ms fails 300 ms**; input p95 **83.341 ms meets 100 ms**. This is a diagnostic subset completion, not full sustained or release acceptance. The current repository is not used as the identity of the preserved run.

## Concrete finding: CLOUD-055 (P2), separate from CLOUD-032

Observer v1 retained 201 rows: **100 Home ready, 100 Define censored with `control-cap`, and one closed lifecycle row**. There are no missing/duplicate stages, sequence gaps, overflow, late callbacks or unresolved active generation. Define never produced a saved-editor ready witness, so its observed-ready→driver lag is unavailable, not zero. Its tiny sampling-gap statistics describe only that pre-navigation capture sample and reveal nothing about subsequent Define readiness or scheduling.

Every Define censor occurs on sample 1, synchronously inside the trusted Open-click capture listener before the product's click handler. The still-current Home tree has enough controls to reach the frozen 32-control bound. In v1, `readDOM` walks the lens before it checks whether the selected Rail project matches the Define target. The controlled original fixture left summary promises pending, so it did not exercise this resolved Home control count. The cap refused excess work as designed; coverage of the requested Define stage is missing. No product defect or slow-render conclusion follows from those censors.

The narrow proposed v2 correction is to check exact selected project identity before scanning the Define subtree. Keep all caps, original actions/spans/thresholds/routes and unknown-context conservatism. A controlled styled fixture must first reproduce the current old-Home >32-control censor and then show a valid fresh Define witness with the gate moved; wrong selection, missing/wrong context and lifecycle cases remain unavailable/censored as appropriate. Frozen v1 and all its original rows remain unchanged. Root reported this instrumentation gap as CLOUD-055 P2; no repository changes are made here.

## Remaining observations

All 100 Home rows can be joined independently to their matching trusted Open click and original browser marker; they do not require a positive Define predicate. Browser-clock intervals are:

| Interval | p50 | p95 | max |
| --- | ---: | ---: | ---: |
| Trusted All-projects click → first observed Home predicate | 16.2 ms | 33.3 ms | 275.5 ms |
| Observed Home predicate → trusted Open click | 74.6 ms | 258.9 ms | 340.6 ms |
| Trusted Open click → original driver's saved-value DOM mark | 69.8 ms | 190.1 ms | 481.1 ms |

The last interval remains the old driver-triggered observation and cannot establish first readiness. Home max sample gap is 275.4 ms; 3 Home stages have gaps >50 ms and 1 >100 ms. Those descriptive gap cutoffs are not acceptance criteria. A late sample with that blind interval does not demonstrate late app readiness.

Original Node command/verification subspans, kept separately from browser intervals, have p95: All-projects command 114.423 ms, heading wait 37.710 ms, Open command 215.644 ms, saved-context verification 103.572 ms, and two-frame command 36.647 ms. These include scheduling/transport/app work, not exclusive CPU timings; percentiles cannot be summed.

Nineteen primary switches exceed 300 ms. The eight largest are fully retained in the projection, including:

- Index 392: total 625.603 ms; Home observed→Open 340.6 ms, then Open→driver DOM mark 145.5 ms.
- Index 136: total 648.400 ms; Home observed→Open 75.8 ms, then Open→driver DOM mark 481.1 ms.
- Index 176: total 892.162 ms; Home is first observed after 275.5 ms, with a 275.4 ms sampling gap, so its early readiness is unknown.

Both sides of Open contain delay in different rows. Fixing the diagnostic's missing Define coverage is the concrete next step; this run supports neither a single product bottleneck nor speculative source optimization. Nothing is subtracted from the original full metric, and no physical paint, first possible readiness, pure driver overhead or causal improvement is claimed.

The separate trace cohort contains 8 switches, 16 inputs and 8 resets. Its switch p95 is 278.377 ms and input p95 79.177 ms. It intentionally has no autonomous observer and does not replace the failing primary cohort.

## Evidence and independent closure

`actual-attempt2/` is the complete allowlisted analysis: all 432 original latency rows with original indices and numeric Node/browser timing fields, all 201 observer rows, 102 cumulative drain windows, every primary switch's join/censor state, grouped summaries, original input hashes and frozen-versus-accepted source comparisons. Private project identifiers are mapped internally to declared owner-order ordinals; names, values, credentials, raw URLs, free-form errors and fixture paths are not published. Original private files are read-only and remain where root retained them.

`actual-attempt1/` preserves the initial analysis. It correctly reported zero Define joins but also withheld Home intervals because its join was all-or-nothing; `actual-attempt2` independently admits the valid Home-to-click interval without treating the censored Define as ready. Six small offline synthetic tests pass, including signed negative lag, cap-censored Define with Home-only observation, mismatched clocks, retained failed durations and private-field omission. No browser or new installed run was launched for this analysis.

`independent-closure.json` records exact journal-bound driver PID 2384364 and controller PID 2384682 absent; Docker containers and volumes empty; engine and image identity matching the run; all bounded original artifacts unchanged. Browser closure, fixture removal and resource counters are identified as driver assertions. No independent browser PID census, whole-machine closure, historical zombie attribution, private fixture inspection or cleanup mutation is claimed.

Root's immediately preceding Rust relocation is retained as environment metadata: 620,087,163 bytes moved from root disk into `/tmp` tmpfs, increasing root disk availability while consuming tmpfs/memory capacity. It is not a whole-host capacity gain. This was not a matched baseline/control trial, so standalone timing differences from earlier runs are not an optimization result.
