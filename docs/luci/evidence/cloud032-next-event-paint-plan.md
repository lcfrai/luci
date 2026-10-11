# CLOUD-032: separate browser presentation evidence from the unchanged full action

Source review pin: `786652704fa8bebd1ba1af41cfa1d414d7def100`. Evidence remains the original b94ad37e eight-round Layout diagnostic and the older 29931afc primary measurement; neither is silently rebased to this source. This is a proposal only. No app, browser, test, profiler or benchmark was launched for it.

The primary action in `scripts/acceptance/sustained-four-project/interaction-metrics.mjs:62–78` includes two normal auto-waited clicks, the overview heading wait, textbox wait, exact context evaluation, independent input-value assertion and two animation frames. Its complete Node interval and 300 ms switch target remain mandatory. The 100-switch primary p95 of 494.921816 ms still fails; the eight instrumented b94 switches at 235.70–354.55 ms do not replace that cohort.

The b94 capture joins 16 trusted Home/Define clicks to their passive DOM witnesses: Home 16.4–37.1 ms; Define 41.6–66.8 ms. No app component was attributed by its Layout stacks. DOM-observed-ready is not first possible readiness, a painted target, compositor presentation or physical display latency. Current project selection is synchronous (`uiSlice.ts:208–218`). Home destinations render ahead of asynchronous overview/capacity content (`ProjectOverviewBoard.tsx:132–138`). Define's initial authority check and saved-context request are real gates, not evidence of an unnecessary network waterfall. No CSS, memoization or component-remount patch is justified by the available trace.

## Small next discriminator

First reuse the existing exact click/EventTiming joins and retained timing windows from `docs/luci/evidence/cloud032-layout-attribution-b94ad37e/attempt2/`. The unchanged observer already records browser `PerformanceEventTiming` startTime, duration, processingStart, processingEnd and interactionId. No new observer or installed run is needed to report those available rows. Retain every one of the 16 Home/Define clicks and the original 32 actions; label this small instrumented cohort explicitly.

Report three separate quantities for each exact joined click:

1. Browser EventTiming's reported event-start-to-next-paint duration. This is a browser presentation measure with quantized timing, not physical screen measurement. The first next paint may show a loading state rather than the target definition.
2. Trusted click to the existing exact project/context/enabled DOM witness on the same document clock. This establishes only the recorded witness; it does not prove target presentation.
3. The unchanged full scripted action and its existing Node command stages. Keep them on their own clock. Do not subtract browser durations from these stages or advertise a corrected acceptance time.

EventTiming uses the existing 16 ms reporting threshold and browser quantization. An omitted entry is censored/unknown, not zero or a successful short interaction. Keep visibility changes, missing/ambiguous joins, repeated interaction IDs, unsupported documents, sequence gaps and overflow explicit. Pair only the exact trusted button event in the original same-document stage window; do not select whichever nearby event gives a favorable duration. No headers, private project names, DOM values or arbitrary trace arguments belong in the derivative; project ordinals and fixed stage labels suffice.

The immediate oracle is whether the retained browser next-paint observations themselves have large tails, versus predominantly larger outer command intervals with short browser observations. It cannot assign all unaccounted time to Playwright, the application or the host. Existing durations and overlapping renderer intervals are wall observations, not additive CPU accounting.

If the existing rows leave a specific long browser-presentation tail, the next separately authorized run should target that named interval only: retain the original action sequence and exact source/package/helper bindings, and collect a bounded renderer/presentation trace joined to that actual trusted click. Attribute repeated script/layout work to an exact product source operation before proposing a product optimization. A generic React commit frame or coincident polling response is insufficient. If the requested trace cannot distinguish presentation of the target context from the earlier loading paint, retain that limitation rather than inventing a target-paint timestamp.

Do not launch another broad profiling or static-page control solely because the overall action fails. Existing settled-locator controls and generic Layout traces already have scoped receipts. An additional capture needs a named unresolved interval and a falsifiable attribution question, without altered actionability, request suppression, response replay or adjusted thresholds. Any new observation adds overhead and stays diagnostic; it cannot produce an acceptance pass by subtraction.

## Independent efficiency correction

CLOUD-078's unchanged-session reconciliation candidate stands on a separate source and controlled-test contract: identical state should not notify subscribers or synchronously rewrite identical preferences. The poller still makes its fresh request and invokes its good-tick callback. Its 27-pass/9-expected-failure baseline and 36-pass fixed test batch are not evidence that this work caused CLOUD-032 or that full-action latency improved.

Full Windows/native and loaded four-project measurement remain distinct from these Linux observations. Repeating the unchanged primary full-action metric after an accepted product change is the only route to changing its acceptance result.
