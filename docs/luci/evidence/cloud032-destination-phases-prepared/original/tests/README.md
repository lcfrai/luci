# Destination diagnostic contracts

`contract-test.mjs` contains 27 prepared contracts and is **not executed**. It imports the prepared diagnostic helpers and transpiles only the overlay probe through the existing pinned TypeScript 6.0.3 `transpileModule`, then evaluates it in isolated Node VM contexts. The suite has a 20-second deadline; VM evaluations have one-second synchronous bounds. It does not launch a browser, build a web bundle, start a service, contact a network endpoint or modify product state.

The tests exercise inert hooks; exact response, generation, revision and textarea binding; evaluation beginning before product readiness; stale or replaced state; first-commit and reset-read behavior; bounded observation and privacy; strict capture completeness; the original immediate/rAF connected-value oracle and ten-second timeout; and the existing interaction await order, `inputValue` assertion and two-frame completion callback.

When execution is separately authorized, use the existing Node runtime:

```sh
node /workspace/.luci-tools/cloud032-destination-phases/tests/contract-test.mjs
```

Success prints one `PASS` line per contract and a final count with hashes of the exact probe, helpers and TypeScript compiler used. A passing VM contract does not prove React integration, browser timing, performance acceptance or a speed improvement. It does not measure retry polling or subtract diagnostic overhead.
