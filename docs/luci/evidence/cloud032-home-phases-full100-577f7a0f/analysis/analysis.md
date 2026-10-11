# Full100 Home numeric diagnostic: no Home/storage optimization justified

All100 Homes are complete:1200 contiguous numeric phase rows, exactly one of each12 phase kinds per Home, successful operations, no active/pending Home, and exactly84 completed-start-entry retirements from the unchanged16-entry map. Every snapshot's raw counter matches max(0,homes−16); no extra drop, overlap or missing phase is accepted. The raw counter is preserved, not reset or relabeled as zero.

Across all100 Homes, handler duration peaks at2.7 ms; handler→Home layout-effect commit at10.0 ms; navigation-state calculation at0.7 ms; complete scoped write at0.6 ms; scope scan at0.3 ms; cockpit serialization/write at0.4 ms; AppShell render-entry→layout-effect at7.1 ms. These are nested spans and must not be summed. No measured Home/storage bottleneck supports a product patch. The previous plain run's larger browser click-dispatch intervals are not measurements of this isolated handler and cannot be assigned to storage.

All400 actions succeed. Instrumented switch p50=214.373930 ms, p95=300.444424 ms, p99=452.113584 ms and maximum528.135568 ms. Input p95=73.000268 ms; reset p95=504.385448 ms. The strict300 ms switch reference is still exceeded. This diagnostic is not plain-web acceptance, and its difference from the failed plain run is not claimed as a gain or used for overhead subtraction.

Six switches exceed300 ms. Four are dominated by saved-context verification, one by the destination click command, and one by the Home click command:

| Switch | Whole switch | Largest original Node span | Specification end → expected-value witness | Orchestrator end → witness |
|---|---:|---:|---:|---:|
|43|380.91 ms|Verification200.82 ms|157.40 ms|139.40 ms|
|54|436.71 ms|Verification261.34 ms|242.20 ms|216.60 ms|
|61|452.11 ms|Destination command262.40 ms|52.50 ms|23.60 ms|
|68|402.98 ms|Verification241.39 ms|220.80 ms|204.90 ms|
|71|528.14 ms|Verification365.52 ms|352.00 ms|326.80 ms|
|92|300.44 ms|Home command122.23 ms|34.00 ms|18.30 ms|

These browser-clock response-to-witness spans are directly measured associations. They do not isolate response consumption, React rendering, locator retry or protocol scheduling. Specification response totals in the four verification tails are only14.4–27.9 ms; their requestStart→first-byte intervals are1.1–2.8 ms. A backend-read change is unsupported by these cases. The specification loader directly awaits definitionApi.get and sets the saved state; it does not await an orchestrator response before doing so.

The precise missing observation is destination readiness. Existing `verify-start/end` surrounds three operations: `field.waitFor()`, `locator.evaluate()` with its correct-value/rAF loop, and `inputValue()`. The browser expected-DOM marker fires only after the locator resolves and the loop sees the right value. There is no retained first control match, specification-consumer completion or DefinitionWorkspace commit marker. The Home probe's AppShell commit is for null project/branch and does not observe destination rendering. Therefore neither application rendering nor automation scheduling can honestly be called the proven remaining cause.

This diagnostic branch is closed without a product patch or another run. The failed plain577 gate remains open. Continue the latest functional package work; do not weaken the benchmark or origin/cleanup guards. If future evidence requires this branch to resume, `future-diagnostic-seam.json` identifies a narrowly scoped response-consumption/definition-commit/locator-boundary seam that preserves the original action and completion metric. It is a recorded future option, not implemented instrumentation or a new execution request.

The capsule retains every original action and captured browser/event row in the strict allowlisted projection, plus all1200 numeric Home rows and100 exact ordinal joins. It excludes project identifiers, storage names/values, draft text, credentials and private fixture contents. Source/driver/web bindings and the unchanged raw SHA256 are explicit. All work here was offline: no browser, app, test or build ran.

Root's separately recorded closure binds the same run. Helper observations show no active owned processes; of three browser PID/start identities, one is absent and two remain same-start zombies. There is no exhaustive lifetime census or zero-zombie claim. The independent closure's original bytes are separately hashed for publication; this timing capsule does not replace it.
