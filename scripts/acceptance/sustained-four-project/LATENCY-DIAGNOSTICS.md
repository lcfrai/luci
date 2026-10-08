# Additive responsiveness diagnostics

The sustained runner captures and rehashes this imported helper alongside its other executed source files. The offline controlled tests do not establish application responsiveness.

The existing primary action start/end, 15-second deadline, action sequence,
verification, number of per-action browser RPCs, two awaited animation frames,
6000-action bound, workload and 100/300 ms input/switch thresholds are unchanged.
Node adds synchronous clock readings at existing await boundaries. Switching
still includes All projects, waiting for the overview and opening a project.

Additional diagnostics:

- Each primary action records Node-clock action/verification/frame RPC offsets.
  Switch actions also record completion of each existing click/overview wait.
  This separates command stages; it does not isolate pure transport latency.
- Trusted input and pointer dispatches record browser timestamps. Input capture
  schedules two passive rAF callbacks; these do not add awaited frames or RPCs
  to the action. Existing expected-value predicates record their first observed
  success using the same call, and the existing two-rAF RPC returns its request
  and callback timestamps. The diagnostic never reads or retains input text,
  key names, selectors, targets or accessible names.
- Same-origin ResourceTiming entries are retained only for exact known API
  route classes. Project/workspace identifiers, raw URLs, queries, request
  headers, bodies and response contents are discarded. The specification,
  overview, orchestrator, queue, run and scheduler classes can explain observed
  network timing; these are not backend function durations. Cross-origin and
  unknown routes are ignored. Zero transfer sizes are not interpreted as proof
  of an empty response. responseStatus is captured only when supported and in
  the HTTP status range, otherwise null. Requests still in flight, absent from
  ResourceTiming or lost/hidden after abort are not a complete request inventory;
  cancellation and server execution remain unknown.

The browser stores at most 2048 pending diagnostic rows per document. Existing
between-round/navigation snapshot RPCs drain both collectors. A separate ledger
allows at most 50,000 rows, 8 MiB serialized snapshots, 2000 windows and 128
documents; the existing overall browser-window 32 MiB cap includes these rows.
Overflow is sticky and retained in an in-budget window before acceptance fails.
Sequence validation covers delivered callbacks, not unobserved browser events.

Clock domains must stay separate: never subtract Node timestamps from browser
timestamps. `references` link only to the latest observed dispatch/expected DOM
witness in that document; unrelated prior inputs or pointer events may be the
latest record, including input in an earlier project within this same SPA
  document. They are not guaranteed causal IDs for the action. Draft reset
has no added expected-DOM marker because its original inputValue verification
is a protocol operation. The action's primary verification remains unchanged.

Two rAF callbacks are scheduling/paint opportunities, not measured physical
display latency or React commit proof. Native input value can precede completion
of React layout work. Input/frame timing and passive EventTiming together can
localize likely renderer/scheduling work, while Node spans include transport,
actionability and host delays. No component profiler or product instrumentation
is installed by this module.

Instrumentation overhead is present in the primary metric: additional clock
reads, small bookkeeping, passive listeners/rAF callbacks and resource observer
processing. Browser cumulative callback count and self-time are retained;
these exclude snapshot serialization, clock overhead and observer/runtime cost.
Node records its added stage clock-read count. Setup adds one init-script RPC
before measurement. Total overhead is unknown; do not subtract these partial
costs or claim comparability with an uninstrumented run without qualification.
No full browser/performance run has yet validated this diagnostic on Luci.

Controlled checks (no browser, service, compiler, model or Docker):

```sh
node --test latency-diagnostics.test.mjs
```

The VM/fake-page test covers privacy, route normalization, sticky bounds,
sequence refusal, unsupported observations, two-frame ordering, closed-record
immutability and the exact existing per-round action/RPC sequence. The real
browser and installed app remain for the root-coordinated final package window.
