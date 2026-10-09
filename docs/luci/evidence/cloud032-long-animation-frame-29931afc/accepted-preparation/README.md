# Exact299 long-animation-frame diagnostic preparation

Prepared externally; **no browser, installed app, Docker, worker, provider, or model run has occurred**. The product/package and declared helper pin are `29931afc5b31ae9b9bd7b8973a7e3fdb840609a3`. This is one targeted instrumentation run, not a product optimization or a replacement acceptance journey. The latest published switch p95 remains579.140ms against300ms; that run's input p95 was97.398ms against100ms.

The remaining question is whether delayed switching episodes coincide with qualifying long renderer frames containing application script/layout work. Existing exact EventTiming joins show that handler duration is not the broad cause; all original navigation predicates matched at their first sample. This observer does not infer a cause for historical rows220/384. A new delayed episode must appear in the new receipt before any causal proposal.

## Frozen workflow and source

`actual/` is a copy of the published5f4 navigation-callback runtime, bound to the matching299 package. All17 selected repository helper files remain byte-identical. The only driver changes are an import, capture registration and one setup hook. `interaction-metrics.mjs`, `navigation-verifier.mjs`, `latency-diagnostics.mjs`, `autonomous-readiness.mjs`, and `readiness-ledger.mjs` are byte-identical to the preserved original. Their original100 switches,200 inputs,100 resets, eight separate trace rounds, all command/verification/frame waits and original markers remain. Targets stay300/100ms p95 and p99 strictly below1000ms; nothing is subtracted.

The setup hook installs one passive `PerformanceObserver` per document and extends the existing diagnostic snapshot/ledger. It adds **no RPC, frame loop, polling timer or product-state action inside any measured span**. Snapshot draining and ledger validation occur through the existing calls outside those spans. No product source, CSS, authority check, backend policy or draft behavior is changed.

The exact package is `/tmp/luci-retained-tools/package-0.5.0-29931afc`, provenance `artifacts/quality-swarm/candidate-29931afc/package/package-provenance.json`. `runtime-binding.json` records its archive/binary/web hashes. Attribution uses170 actual packaged JavaScript files,9,237,001 bytes total, through a path→SHA256 map. The full package/archive checks remain in the launcher; preparation did not run them again. The retained76043c runtime image's106 input paths have no source change from5f4 to299; no Docker inspection was performed during preparation. Existing before/after image/package/source/dependency and exact owned-process cleanup rules are preserved.

## Output and exact join

Every existing passive browser snapshot receives `latency.longAnimationFrames`. The Node receipt also retains the validated, bounded windows under `longAnimationFrameCapture`. A frame contains its sequence and numeric browser-clock start/duration, blocking duration, render/style-layout start and first UI timestamp. Scripts contain only numeric execution/duration/forced-layout/pause fields, safe categorical enums and, for an exact known same-origin asset with no query/fragment, its bundleSHA256 and source-character position. Source URL, function name, invoker string, DOM, input values, headers, body content and screenshots are not copied into this observer.

`long-frame-joins.mjs` consumes the original exact200 primary click EventTiming joins produced by the existing published analysis. It validates same timeOrigin, unique exact click containment, round/project ordinal/stage identity, then links overlapping retained frames to:

1. The browser-reported click interval.
2. Trusted click→autonomously observed ready interval.
3. For Open, prior Home observed-ready→trusted Open click, to inspect the pre-dispatch interval without inventing a Node/browser clock conversion.

It retains **all frames**, including those outside these windows, and every original click join. It refuses invalid capture, ambiguous/mismatched identities, duplicate rows and incomplete click counts; unavailable original joins remain explicit. It reports unobserved documents and intervals beyond the final snapshot instead of treating them as zero work. The existing exact-click analysis is reused unchanged; its original raw-result and source hashes must accompany the new projection.

```sh
node /workspace/.luci-tools/cloud032-long-frame-29931afc/long-frame-joins.mjs --result <new-private-result.json> --clicks <new-exact-event-timing-click-joins.json> --output <fresh-safe-long-frame-joins.json>
```

## Bounds, lifecycle and limits

Each document is bounded to2048 frames,8192 script records,128 scripts per frame and2MiB of retained UTF-16 row accounting. The Node ledger is bounded to4096 frames,16384 scripts,4MiB encoded windows,16 documents and2000 windows. Existing original receipt/latency caps are unchanged. Unsupported/failed observer installation, missing/wrong-document data, bounds, sequence/counter regressions or unavailable capture cause explicit diagnostic refusal; an in-budget failure window is retained before refusal. A failed diagnostic is not repaired or relabeled complete. Pagehide flushes pending records and disconnects; late callbacks cannot append data.

The browser API is selective: it reports qualifying long animation frames and attributable scripts, not every task, render, paint or subthreshold script. Empty matches are **not proof of an idle renderer**. A snapshot timestamp is not a continuous rendering-completion watermark. These durations are wall-clock intervals and can include descheduling; they are not renderer CPU time. No CPU Profiler is used; earlier negative-delta profiles remain invalid. The observer cannot by itself prove host scheduling or protocol settlement as the cause, or assign React work to a component when only a shared bundle position is exposed.

Callback counts and callback-body self-time are retained as partial overhead accounting. They exclude browser observer machinery, URL classification outside sampled callbacks where applicable, clock reads, snapshot serialization, Node validation, protocol transport and changes to scheduling. No overhead is subtracted. The extra trace remains a separate eight-round cohort, with different instrumentation. Actual raw Playwright trace and full private fixture/result remain private; any publication uses an explicit allowlist.

## Controlled preparation checks and launch

`inert-attempt1/` preserves12 successful observer/ledger controls. `inert-attempt2/` preserves the combined16 successful controls, adding exact joins and refusal cases. Inputs were unchanged during each batch; no tests were skipped. These are Node vm/fake-observer controls, **not proof of real Chromium API availability**. They cover original snapshot preservation, one setup call, init ordering, drain/sequence, unsupported/install failure, bounds, malformed times, source privacy/binding, pagehide/late data, existing-ledger failure, exact joins and absent coverage.

Three additional exact-source integration controls pass in `integration-inert-corrected/`: the driver has only its three setup changes, all original action modules remain unchanged, captured dependencies match, and driver/launcher parse without execution. The original2-pass/1-failure batch and exact test are retained in `integration-inert/`; that failure was a test-only `Array.index` typo, corrected to `indexOf`. Runtime/helper bytes did not change. Final scope is **19 unique successful inert controls**, not a sum counting repeated runs.

The prepared live command is the exact array in `launch-command.json`. It requires explicit root window release, the explicit Unix Docker endpoint, an empty selected daemon, fresh output, unchanged sixGiB root floor and exact package/image/helper/dependency bindings. Current output is `artifacts/quality-swarm/idle-29931afc-long-frame-attempt1`; do not reuse it if created. The original60s preparation,210s workflow,120s cleanup and bounded outer-driver TERM/KILL settlement remain. **Do not launch during another installed window.** If checkout changes beyond the launcher's already supported docs-only descendant rule, rebind an explicit helper revision before runtime; do not bypass its guard.

The repository remains untouched. `manifest.json` and `SHA256SUMS` freeze the external prepared source, commands and inert receipts. Readiness means source prepared and inert controls passed; installed results remain unrun.
