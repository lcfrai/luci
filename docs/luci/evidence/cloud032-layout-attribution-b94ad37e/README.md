# CLOUD-032: bounded Layout caller attribution, b94ad37e

The second capture completed all eight diagnostic rounds and 32 original actions. It does **not** establish a latency pass or identify an app component responsible for the earlier slow switching. The original 100-switch p95 of **494.921816 ms fails the unchanged 300 ms target**; the same original primary run's 200-input p95 was 86.268305 ms against 100 ms. These eight instrumented rounds do not replace that cohort or support a before/after comparison.

## What the actual capture shows

V2 contains 70,958 raw events, 1,926 allowlisted layout/invalidation events, and 952 Layout/style events. All 16 Home/Define trusted clicks join exactly to the browser EventTiming records on one document/renderer clock. The 16 marker offsets span 81 microseconds. Maximum reported trace-buffer use was 53.67%, with `dataLossOccurred:false`, stream EOF, stream close and session detach acknowledged. The private raw trace is 28,383,461 bytes; its exact hash is retained below and in the receipts.

The 16 Home/Define readiness windows contain 41 Layout and 70 UpdateLayoutTree events. Only 16 have attributed stack positions: one style/layout pair in each Define window. Every attributed caller lies in React's commit/deletion/scheduled-work path. No app component caller is identified. The longest Layout in these windows was 7.243 ms; longest style update was 9.046 ms without a stack. The overall 14.622 ms style update maps to Radix FocusScope focus during draft reset, outside switching windows. Changing accessible dialog focus is not supported by this switching evidence.

| Existing timing scope | V2 observed range |
| --- | --- |
| Home trusted click to passive DOM-ready witness | 16.4–37.1 ms |
| Define trusted click to passive DOM-ready witness | 41.6–66.8 ms |
| Full original project-switch action | 235.70–354.55 ms |
| Longest React scheduled-work FunctionCall per switch window | 8.639–21.959 ms |

Full actions retain both original clicks, expected content assertions and two animation frames, including automation transport/actionability and verification. The click commands themselves account for separate original Node intervals of roughly 71–121 ms and 80–124 ms. These wall-clock intervals include different work and must not be subtracted from browser intervals, added as CPU costs, or treated as physical input-to-display latency. Instrumented intervals do not identify how much of a click command was transport versus Playwright actionability. The earlier 91.8 ms forced-layout outlier was neither reproduced nor explained by V2.

`attempt2/analysis/all16-stage-joins.json` retains every joined stage and overlap; `all-layout-style-events.json` retains all 952 events including reset work; `all-renderer-function-calls.json` retains all 5,633 renderer FunctionCalls with known asset positions or explicit unattributed status. `all-interactions.json` retains every original action row with only fixture project identity replaced by an ordinal. No row filter, calibration subtraction, metric threshold change or performance attribution is applied.

## Failed attempt1 remains a failure

Attempt1 refused with `trace-buffer-limit` after one completed/two attempted rounds. Eight action rows happened to complete before the in-flight work stopped; only the first round has both boundary markers. The 32,234,319-byte private trace reached 83.05% reported buffer use against the unchanged 75% guard. `dataLossOccurred:false` does not override that refusal.

Fine `disabled-by-default-devtools.timeline.invalidationTracking` records account for 24,948,139 event-JSON bytes (about 77.4% of the raw file). All nonmetadata category groups intersected requested categories; default-only leakage was not observed. V2 removes that fine category and adds explicit `excludedCategories:['*']`. The remaining four categories retain coarse layout/style invalidation events and stacks. This deliberately narrows diagnostic detail; it is not a claim of equivalent captures or that default-category leakage caused attempt1. All original limits remain unchanged. The exact two-line source change and affected controlled-test receipt are retained in `accepted-preparation-v2/`.

## Product, helper and live checkout identities

Both attempts used product/package and selected helper pin `b94ad37e53181e360ac8910ee980570de622728c`. The original interaction modules are unchanged; only external eight-round capture orchestration differs from the primary diagnostic. The frontend web bytes match 29931afc. V2's actual clean checkout was the docs-only descendant `e780abe20cc6d67096c00e901578716afbb2cb51`, explicitly recorded separately from the selected product/helper source. Exact selected helper files and package members were verified before and after execution.

- Package archive SHA-256: `4a5ab15e2cbb926f6ad70609507429641dc65ee4719ef462c50df96f04cffe0f`.
- Binary SHA-256: `c9816ced2db72fb508ccbcb2880c55f36049ca0d581c66fafc8a40bd65ca5a66`.
- Index JS asset SHA-256: `526b6f92ddac31da933f04978f13f5bd5a3e2e5312422a04d89b8c53051c9c3e`.
- V1 raw result SHA-256: `cce8e1fa7195525e2bf4976cee45844876cf62cf2de5d5960ee88394cb99645f`.
- V1 private trace SHA-256: `45463172fffdaa9503bfb6bddb78e8dde13446305c60327362e7e6151974d763`.
- V2 raw result SHA-256: `fead13b770f2351282fc4c9a1bd9d0ab34248b087311e9b4ccdb651ed424fe26`.
- V2 private trace SHA-256: `f55eb46a4fc0d6ece3a2393135fb3636aaf5b5548f7db41eea3931b50659aa3c`.

No hidden source-map build or runtime source mutation was performed. Bundle positions are matched to exact emitted bytes and available framework source correspondence; a framework wrapper is not a component attribution. Exact preparation trees are retained without changing historical `readiness.json` fields. Their final controlled checks were V1 39 checks and V2 23 affected checks. The unchanged 17 projector checks are not counted again as new V2 executions; earlier failures and preparation attempts remain alongside their corrections.

## Bounds and closure

Both runs preserve the original 60-second quiet phase, four fresh paused project contexts, normal UI actions and hard zero Docker Create/Start guards. V2 observed zero Create, Start, external model and provider calls. Trace bounds remain 30 seconds, 32 MiB record-until-full buffer, 75% stop fraction, 64 MiB raw output and fixed command/drain/cleanup limits. V2 wrapper elapsed time was 102.628590 seconds; exit0 means diagnostic capture completed, not performance acceptance. No Linux sampler was active. Trace/marker/passive-observer overhead is not calibrated and must not be subtracted.

Independent read-only closure for **each** attempt found three recorded identities absent and two recorded browser identities as PID1 zombies; Docker containers and volumes were empty. This is a known-owned-identity check, not a whole-machine census. The declared fixture parent still contains the same hash-identified failed attempt1 fixture. V2's own fixture was removed. Wrapper cleanup and independent closure are separate receipts; zombies are not silently relabeled absent.

## Reproduction and privacy

The exact executed command arrays are `accepted-preparation-v1/launch-command.json` and `accepted-preparation-v2/launch-command.json`; each frozen launcher validates the selected source, package, image, imports and bounded lifetime. They contain the original absolute staging paths and consumed output destinations. To reproduce, restore the corresponding frozen tree to its recorded external path, verify its SHA256SUMS, arrange the exact package/image/dependencies, and supply a **fresh** output/fixture location in a separately hash-bound invocation. Do not rerun the consumed command against retained evidence or reuse the failed fixture. Runtime execution requires a separately coordinated exclusive window. No runtime is launched by this publication.

For retained-input read-only reproduction, run `python3 attempt2/analysis/analyze-v2.py` in an external copy with the original private inputs/package available at the paths declared in that script. This verifies their hashes and reproduces the all-row derived timings. `attempt1/analysis/analyze-partial.py`, exact projector source, partial derivations and both independent closure recipes are retained. Closure recipes query live identities and Docker; their original receipts are historical observations and must not be overwritten by a later observation. `publish.py` documents the allowlisted selection and exact known-value privacy scan.

Raw trace and raw result files remain private. Public data excludes raw URLs, network headers, account details, fixture identities, arbitrary trace args/function names and response bodies. Only exact known asset paths/hashes/positions and bounded event metadata are emitted. Known captured credentials and fixture identities are scanned without emitting the values; `privacy-review.json` records counts and matches. This finite scan is supplementary to explicit field allowlisting and does not prove the absence of every possible secret. The original private bytes remain hash-bound and unmodified.

No app performance patch is justified by these stacks. CLOUD-032 remains open against the original complete primary measurement. The source-path review can identify independently provable redundant work; this capsule does not license a speculative rerender, CSS or focus change.
