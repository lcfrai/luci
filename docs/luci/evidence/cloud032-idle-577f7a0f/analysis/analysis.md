# Packaged577 full idle sample: capture complete, switch target failed

All400 actions succeeded. The100 project switches have p50=218.225628 ms, p95=497.324950 ms, p99=582.330910 ms and maximum583.791429 ms. Eleven exceed300 ms. Input p95=87.794101 ms passes its100 ms target; input p99=359.491735 ms remains below1000 ms. The100 draft resets have p95=568.090839 ms; they are retained as extra actions, not mixed into switch/input percentiles. Wrapper exit0 and capture `passed:true` are not performance acceptance.

The retained switch tail does not identify one product function. The largest original Node span is All projects click in five slow switches, destination click in three, and saved-context verification in three. Overview-heading wait reaches only30.37 ms; browser frame-request to second callback reaches33.3 ms across switches. Every original row and every captured browser/event row remains in `timings.all400.projection.json.gz`; Node stage breakdowns for all400 are also provided.

Two destination cases are particularly useful:

| Switch | Whole switch | Destination command | Specification total | Before requestStart | RequestStart → first byte |
|---|---:|---:|---:|---:|---:|
|49|497.32 ms|291.84 ms|199.50 ms|170.10 ms|27.90 ms|
|58|583.79 ms|350.03 ms|298.80 ms|283.40 ms|11.00 ms|

The paired orchestrator requests in those cases also wait177.6/282.0 ms before requestStart. Switch87 has five Home scheduler/overview requests with192.4–193.2 ms before requestStart. These measurements do not establish slow specification server reads. The browser guard source pauses every request through Playwright `context.route('**/*')` and `await route.continue()`, but the retained data does not isolate that path from browser scheduling, connection setup or a connection-slot queue. Removing that guard is not proposed.

The retained connection evidence is insufficient to establish HTTP/1.1 slot contention. During the delayed49/58 pre-request intervals, at most two captured route-class resources are started but not ended; at87, at most five. No earlier captured route resource is outstanding when49/58 specification begins. Captured earlier response endings precede delayed requestStart by265/352 ms and by309–312 ms at87, so they show no visible release coincidence. However, the collector omitted protocol, connect/DNS/fetch timings, connection IDs, unclassified same-origin requests and requests that never finished before capture. Six-slot contention is neither proven nor ruled out. `connection-queue-coverage.json` retains exact counts and absent fields.

Home itself has measurable tail behavior that the eight-round probe missed. Uniquely associated click-event processing lasts82.7 ms at switch96 and72.7 ms at99, versus a17.7 ms p95 and22.9 ms maximum among the89 switches at most300 ms. This is browser event dispatch, not an isolated React handler measurement. The previously measured eight Home handlers ≤2.1 ms and commits ≤9.7 ms cannot be generalized to these later events.

Other action types also have tails: eight individual inputs exceed100 ms, with dominant action, verification or two-frame-command spans. For three slow inputs, the browser's own frame-request→second-callback interval reaches156.9,140.2 and207.2 ms. This establishes that delays are not confined to Home alone, without identifying their cause. All input/reset stages remain in `node-stages.all400.json`; no values are subtracted across clocks.

The approved next test is one full100 run of the existing numeric Home web, unchanged original action semantics and thresholds. It will bind each slow Home to actual handler, handler→commit, navigation, scan and persistence spans. That driver is separately frozen as `cloud032-home-phase-577-full100`; no new product optimization, routing instrumentation or guard weakening is included. Until those actual tail spans are available, neither a storage/backend fix nor request-burst/cancellation change is justified by this run.

The private raw receipt SHA256 is `92c533ae6cecb888be68f1644501d7e232c93dc490a8e0cb76e3b046dd102793`; it was verified unchanged after offline analysis. Package source is577f7a0f, helpers are frozen64829123. Product/package identities are reported from the original completed receipt; metric/helper source bytes were independently hash matched. This analysis launched no app, browser, test or build and changed no original evidence, fixture or ledger.

The helper observed all five managed handles closed and no active fixture-owned processes. Of three browser PID/start identities, one is absent and two remain same-start zombies. This is not an exhaustive lifetime census or a zero-zombie claim. Root's independent closure is a separate receipt and must accompany a full runtime publication.
