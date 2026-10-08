# CLOUD-032: switching latency diagnosis and absent-draft cleanup

The fresh installed idle diagnostic still missed the **300ms switch p95 target:367.20ms**. A minimal static DOM control measured157.08ms. These are end-to-end automation timings; no cost has been subtracted. The original sustained failure remains preserved in [the attempt3 capsule](../cloud031-sustained-4df94ab9-attempt3/README.md).

| Witness | Primary switches | Switch p95 / p99 | Primary inputs | Input p95 / p99 |
| --- | ---: | ---: | ---: | ---: |
| Original sustained idle, product4df/helper5d | 100 | 523.94 /713.25ms | 200 | 120.05 /250.75ms |
| Static DOM control | 100 | 157.08 /225.69ms | 200 | 52.54 /94.45ms |
| Fresh installed idle, product4df/dependency9121 | 100 | **367.20** /514.74ms | 200 | 83.22 /207.77ms |

Each diagnostic also performed100 measured draft resets. All400 primary actions succeeded functionally in each arm. Both retained the original action, expected-content verification, final two animation frames,15s action deadline and100/300ms targets. The original partial loaded74 switches/148 inputs did not finish its required200/400 cohort. There is no new loaded or sustained pass here.

The static run completed2026-10-08T22:22:46.127Z; the actual run completed22:27:19.119Z,159.585s total. Each then retained eight separately labelled trace rounds, adding32 actions after the primary400. Trace timings do not replace the primary results. The actual receipt's `passed:true` records completion of this diagnostic; its switch `withinReference:false` retains the latency miss, and `fullSustainedAcceptance:false` is explicit.

## What the traces establish

Both follow-up traces recorded40 clicks and40 first attempts, with **zero retry, unstable-element, intercepted-pointer or detached-element messages**. The actual All-projects input box was identical across eight visits; each Open-project box matched across its two sampled visits. These are input-time samples, not continuous geometry history. The original sustained run had no retained Playwright actionability trace.

Actual traced All-projects calls reached105.83ms, with stable-wait maximum35.05ms. Open-project calls reached123.56ms, with stable-wait maximum33.71ms. The slowest Open locator took62.39ms to resolve, followed by11.68ms before the click attempt. Actual click-execution spans reached31.20ms for All projects and29.16ms for Open project. Static locator spans were approximately1.5–2.9ms. These observations show additional locator/browser/event work; they do not identify a React function, layout routine or synchronous storage write as its cause.

Actual primary switch subspan p95 values were action270.11ms, verification83.60ms and final-frame RPC40.05ms. The nested click p95s were126.13ms for All projects and137.68ms for Open project. Separate quantiles are not additive. The largest switch572.10ms included a specification resource100.20ms and orchestrator resource188.40ms; they completed168.70ms and77.10ms before the expected-DOM witness. All retained known-route responses in those five largest switch windows were200. Resource observations omit unrecognized, aborted and still-in-flight requests and are not backend function spans.

Home remounts start four overview reads and a capacity read. Async card content can change grid-row height, but the small trace did not establish a movement retry or transition delay. No CSS timing change follows from this evidence. The existing rail selects a project directly, restores its saved lens and keeps the dirty-draft guard, bypassing Home's overview reads. That different journey has no measured latency result here and does not replace the original two-click journey.

Both new primary arms omit the whole-process Linux sampler and retain the same original browser collectors. Original observer interpreter cost was80.199CPU seconds/440.749wall seconds (~0.182core), excluding helper/daemon work. Original browser callback self-time was409.4ms; static30.9ms and actual217.1ms cover different documents/work counts. These are partial accounting values, not overhead corrections. The new input result does not establish that removing the Linux observer caused an improvement.

## Narrow source change and validation

Clean editors call `setUnsavedProjectDraft(id,null)` on mount and cleanup. The old action always cloned/set the draft map, invoking Zustand's preference persistence even when the ID was already absent. The change returns before `set` for that exact case. Actual additions, updates, removals and navigation guards keep their existing behavior.

`product-change/` retains the original source, resulting source, patch, three new regressions and raw test report. **19/19 actual store tests passed** across `draftRegistration`, `sourceNavigation` and `projectNavigation`, with one Vitest worker and no file parallelism. They verify zero notification/storage writes on absent clears, real add/update/remove publication, transient draft data, dirty navigation and late cleanup after confirmed discard. The retained jsdom canvas warning did not fail a test. No package or post-change browser latency measurement is included; the new package still requires matching idle/sustained verification.

## Identity, bounds and cleanup

- Product/package: `4df94ab9dad0e9fedd28a873bbadc56720e1402e`.
- Exact original interaction/diagnostic template: `5d306b6dae3cc9d0a4d9cbf95f8142c37958964f`.
- Clean actual dependency checkout: `9121b5bab09df5827efa291026ed842253256842`.
- Interaction helper SHA256: `ec0bc9d06fda6636cbba6975e05b9c66979ca189b59aa38c373b37a318b79502`.
- Browser diagnostic helper SHA256: `a2d787493f0eeaeb95570c7f211ee69c649ff1b7247feec171f2118ac3a42c96`.
- Static driver SHA256: `b9581cc3460ca4034025906f9b0afa7fe4f2852ed5346263395846f3d04a1f98`.
- Generated actual driver SHA256: `d7fb7586e5b4a2639da9abb6c2e1c6a1296d2161ea7cf2ec393a8f1d3ab1b016`.

The static bound was180s workflow+20s cleanup; actual60s preparation+210s workflow+120s cleanup. Both used one headless Chromium at1440×1080, the original60s quiet interval and ordinary clicks. The actual arm used fresh account/state/source fixtures and paused queues. It performed read-only Docker readiness/image/inventory checks, with unconditional guards refusing Create and Start before forwarding. Six-GiB admission and one-GiB running disk reserve were retained.

Actual transport counters were Create0/Start0/worker0/fence0, provider calls0 and auxiliary calls0. All controller/browser/provider/proxy cleanup phases completed, with zero remaining owned processes, containers or volumes; the fresh private fixture was removed. Static cleanup completed in418ms. A subsequent independent Linux process scan found no diagnostic drivers, marked processes, visible Chromium or actual fixture children. Pre-cleanup checkpoint files are retained as intermediate evidence; final receipts and closure observations determine completion.

## Reproduction and publication scope

`static/accepted-harness/` and `actual/accepted-harness/` contain the exact executed bytes. `preparation/prepare-idle.py` generates the actual driver through15 exact, checked changes to the preserved original. Its block hashes and diff are retained. It stops after the original idle cohort, denies dispatch/Create/Start and puts browser cleanup before proxy drain. The copied original task-command parser is unreachable in this idle subset; this derivative is not the corrected full sustained driver.

The exact command arrays are in each arm's `invocation.json`. Use a fresh output directory and private fixture; never reuse a failed authenticated fixture. The original helper expects a clean checkout and the retained verified package/image paths. To reproduce elsewhere, restore a clean9121 checkout and package4df inputs at the declared paths, recreate `original/` beside the exact generator in a fresh external directory, run the generator, and use its resulting `actual/` directory. Its new absolute execution path is recorded in a fresh binding; the generated driver and original timing bytes remain exact. A changed dependency refuses execution. Run the static command first and coordinate an exclusive browser window before the actual arm. Both commands and requirements are also preserved verbatim in `preparation/original-preparation-README.md`.

Raw result JSON is gzip-compressed without changing decoded bytes. `capture-bindings.json` binds the original files and decoded/compressed hashes. The actual raw trace is **excluded**: it remains private, SHA256 `4b9e9e6922bea8360e6690ecd22efaa35e0c4f7b42e175af47fb83ac3d0ccd5d`. An indicator scan found no authorization/Bearer/password/bootstrap/API-key/token markers; absence is not proof of no credentials. Only allowlisted numeric timings/boxes, categorical methods, counts and hashes are published from it. The static trace contains only the authored DOM fixture and no authenticated app. No auth/private fixture is published or reused.

`analysis/analyze-pair.py` records the allowlisted extraction; `analysis/comparison.json` contains the derived spans and network windows. `analysis/publication-review.json` describes the review boundary. No physical input/display, live-model, cloud, Windows or post-change latency claim is made.
