# CLOUD-032: existing b94 click, DOM and full-action observations

This is an offline derivative of the completed eight-round b94 diagnostic. It retains all **16 trusted Home/Define stages and all 32 original actions**. It launches no application, browser, profiler or test and changes no repository file, observation, threshold or original receipt.

The existing rows do **not** identify a product performance fix or a substantial browser presentation tail that warrants another broad capture. The largest reported click duration is 80 ms; the largest reported interaction-group duration is 88 ms. These differ from the complete scripted switch durations, whose range is 235.701192–354.552370 ms. The difference between those quantities is not attributed to Playwright, the application, the host or any other cause. No cross-clock or overhead subtraction is performed.

The measured product and selected helper are `b94ad37e53181e360ac8910ee980570de622728c`, using Chromium `151.0.7922.173`. This is not a measurement of the current product or CLOUD-078. The original primary 299 switch p95 of 494.921816 ms still fails the unchanged 300 ms target. Its 200-input p95 was 86.268305 ms against 100 ms. The eight instrumented rounds here cannot replace that primary cohort or establish a speedup.

## Complete observations

All values below are milliseconds. The full action is repeated for its two constituent clicks for readability; it is counted once per round in the eight-switch summary. The group column is the maximum reported duration of the retained pointerdown, pointerup and click entries sharing the same document and nonzero interaction ID, not their sum or page INP.

| Round | Stage | Event sequence | Click duration | Group maximum | DOM witness | Full Node switch |
| --- | --- | --- | --- | --- | --- | --- |
| 0 | Home | 1444 | 80 | 88 | 28.8 | 354.55 |
| 0 | Define | 1466 | 56 | 56 | 62.6 | 354.55 |
| 1 | Home | 1570 | 64 | 64 | 37.1 | 342.14 |
| 1 | Define | 1592 | 56 | 56 | 66.8 | 342.14 |
| 2 | Home | 1741 | 48 | 48 | 22.6 | 312.31 |
| 2 | Define | 1763 | 56 | 56 | 49.5 | 312.31 |
| 3 | Home | 1912 | 48 | 48 | 24.7 | 261.17 |
| 3 | Define | 1934 | 48 | 48 | 51.1 | 261.17 |
| 4 | Home | 2070 | 64 | 64 | 19.7 | 293.97 |
| 4 | Define | 2092 | 72 | 72 | 64.3 | 293.97 |
| 5 | Home | 2218 | 64 | 64 | 21.9 | 259.18 |
| 5 | Define | 2240 | 64 | 64 | 41.6 | 259.18 |
| 6 | Home | 2389 | 48 | 48 | 21.5 | 292.58 |
| 6 | Define | 2411 | 56 | 56 | 54.1 | 292.58 |
| 7 | Home | 2559 | 48 | 48 | 16.4 | 235.70 |
| 7 | Define | 2581 | 48 | 48 | 46.6 | 235.70 |

The JSON preserves the full original precision. Home click p50/p95 are 48/80 ms; Define click p50/p95 are 56/72 ms. Home DOM-witness p50/p95 are 21.9/37.1 ms; Define DOM-witness p50/p95 are 51.1/66.8 ms. The eight full switches have p50/p95 292.580964/354.552370 ms. With eight observations, nearest-rank p95 and p99 equal the maximum. All 16 input and eight draft-reset actions are also retained, with separate summaries; none are dropped to improve a percentile.

Recorded click-dispatch handling spans 15–34 ms for Home and 10.9–23 ms for Define. The derived post-dispatch remainder spans 20.8–47.8 ms and 28.4–48.7 ms respectively. That remainder is `startTime + rounded duration - processingEnd`, all on the same document clock. It is not separately measured paint work, CPU cost, or application work.

## Join, omission and clock rules

The original autonomous listener accepts only trusted clicks on the exact Home/Open button ancestry and labels. EventTiming itself retains neither `isTrusted` nor the target. This analysis joins the original listener's `clickAt` to **exactly one** `name == click` record on the same `timeOrigin` whose inclusive `processingStart..processingEnd` contains that timestamp. It does not choose a nearest or favorable event. Event identity is `(timeOrigin, sequence)`; `interactionId` only groups related events. Home's project ordinal identifies the next scheduled destination, not the project being exited.

The original expected round, project ordinal, authority generation, observer generation and document identity are checked. All 16 joins are unique, match the prior independent join table and retain separate interaction IDs. All 16 DOM rows are ready; there are zero missing, ambiguous or censored stage rows in this capture. The script retains those statuses if encountered rather than treating them as zero durations. All 3,799 EventTiming rows across 18 snapshots and ten documents are checked for sequence continuity and original counters. No gap, drop, unsupported window, overflow or late readiness callback is reported. The final closed observer row is retained separately.

The collector uses `durationThreshold: 16` and browser quantization. Missing records would remain censored/unknown: absence would not establish a short duration or pass. All initialization/drain visibility snapshots say visible, but **continuous foreground visibility and per-event visibility were not recorded**. The source has no visibility-change history.

The reported duration is an event-start-to-next-paint observation, with a rounded endpoint. It does not identify target-context pixels, compositor presentation or physical display. A next paint may show a loading state. The DOM witness is an observer sample, not first possible readiness or target paint. Several Define witnesses occur after the rounded event endpoint; others occur before it. No rule forces them to coincide or clips their order. Node stages remain on their own clock, including both normal auto-waited clicks, heading wait, field/context assertions and two animation frames. Two rAF callbacks do not prove paint.

## Bounded next step

No CSS, React memoization, focus or component-remount correction follows from these observations. Existing Layout attribution names React/framework wrappers, not a responsible product operation. CLOUD-078's no-op store-write correction has an independent source/test contract and receives no latency credit here.

If an explicitly authorized presentation investigation is still needed, its narrow target is **round 0 Home, event sequence 1444, interaction ID 1607**, the existing maximum (80 ms click, 88 ms reported group maximum). The concrete question is whether the interval after `processingEnd` contains attributed renderer work or frame scheduling/presentation delay. The next step should first inspect the retained trace for that named interval, rather than rerun the application. A product fix would require repeated work attributable to an exact product operation; a framework wrapper or overlapping asynchronous request is insufficient. If the retained categories cannot show the relevant presentation boundary, report that field unavailable. The 88 ms observation alone does not justify a new runtime trace, and this small cohort cannot rule out tails in other workloads.

## Reproduction and binding

From this directory, with a fresh output path:

```sh
python3 analyze.py \
  --evidence-root /workspace/luci/docs/luci/evidence/cloud032-layout-attribution-b94ad37e \
  --output /tmp/cloud032-event-paint-FRESH
```

The script uses only the nine explicitly pinned published files; it verifies every hash and enforces input byte/count bounds. It does not need a repository checkout at the old product HEAD or the private raw trace. Optionally, `--original-result` verifies the retained private original's expected hash and its exact correspondence with all four published timing fields and the 32-row ordinal projection. The executed analysis performed that check: 10,150,948 original bytes, SHA-256 `fead13b770f2351282fc4c9a1bd9d0ab34248b087311e9b4ccdb651ed424fe26`. The original was not copied or modified.

`derived/input-bindings.json` binds the exact published sources and records original verification. `derived/all32-original-interactions.json` is byte-for-byte the previously published complete action projection. `derived/all16-click-joins.json` retains all candidates, trusted witnesses, same-interaction records and the original Node stages. `derived/observation-coverage.json` retains snapshot/sequence/visibility coverage. `derived/summary.json` keeps unmodified original target declarations, including their diagnostic-only, unassessed status. Raw analysis stdout/stderr and the exact command are retained. No private project names, context values, fixture locations, raw URLs, headers or trace arguments are newly exported.
