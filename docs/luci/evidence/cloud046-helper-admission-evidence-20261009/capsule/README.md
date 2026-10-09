# CLOUD-046 helper: retain the exact refusal operands

This is an additive two-file helper correction after frozen CLOUD-045 commit `e2abd0e39e934e68ead3fd3e906a79336a8bda19`. The earlier CLOUD-045 capsule and all original sustained-attempt artifacts remain unchanged. Root owns integration and publication. No commit or runtime attempt was made by this task.

Once the existing queue/run/session identity guards validate the exact returned session, the helper optionally preserves its `startAdmissionEvidence` in `queueRefusal.session.startAdmissionEvidence`. The seven allowed fields match the separate backend contract: `metric`, `observedAt`, `decidedAt`, `observedPercent`, `maximumPercent`, `observedBytes`, and `requiredBytes`. Unknown fields and nested data are omitted. Valid evidence sets `admissionDecision.status` to `retained` with an explicit actual-decision basis. Missing, malformed, unauthorized or mismatched data remains `unavailable`; no operands are reconstructed.

The helper requires an exited session and an admission failure with the corresponding CPU or memory reason. CPU operands must be finite and within their percentage bounds, with observed utilization strictly greater than the maximum. Memory bytes must be safe nonnegative JavaScript integers, with observed bytes strictly below a positive required amount; zero remains a measured value. Byte integers beyond `Number.MAX_SAFE_INTEGER` are unavailable instead of being presented with lost precision. Unused operands must be zero.

UTC RFC3339Nano timestamps are validated and compared as integer nanoseconds. The observed sample must precede or equal its original decision and be no more than five seconds old at that decision, including the exact boundary. Today's time does not invalidate valid historical evidence. The separately retained scheduler resources remain labelled later and non-atomic; their values are never substituted for the actual refusal operands.

The first durable queue checkpoint still precedes all diagnostic awaits. The same parallel three-second GETs, failure outcome, one-worker ceiling, exact enqueue/session/context guards, no-replay behavior and cleanup path remain intact. No passing task or acceptance result is inferred from a refused attempt.

Final validation used identical updated test bytes against both helpers:

- Frozen CLOUD-045 helper: 54 cases, 46 passed and eight expected new capture failures, exit 1.
- Revised helper: all 54 passed, zero skipped/cancelled, exit 0.
- Helper syntax and scoped `git diff --check` passed; bound source, tests, fixture, lifecycle helper and backend evidence contract remained unchanged during validation.

The new cases cover valid historical CPU evidence, both memory metrics with measured zero, missing/incomplete fields, unknown enums, nonfinite/out-of-range/unsafe numbers, exact five-second and one-nanosecond-over boundaries, malformed/future/zero timestamps, wrong failure stage/reason/status, foreign session identity and omitted private extras. All 46 prior C045 cases remain present. A separate read-only review found no actionable privacy, identity, time or numeric defect. The reviewer made no edits and ran no tests.

These are inert GET/context/loop tests. No controller, browser, Docker, network, external provider or native workload was started. The backend tests and persistence implementation are a separate capsule. This helper batch does not establish an installed retest, UI rendering, native-platform acceptance or performance improvement.

`validation.json` binds exact commands, raw logs, exits, counts, versions, source hashes and before/after input hashes. `candidate.json` binds the two-file patch against the frozen C045 baseline. `before/` and `after/` hold the tested sources; the retained queue fixture is copied under `after/fixtures/`. To reproduce from this directory (the original source is expected to exit 1):

```sh
LUCI_SUSTAINED_TEST_SOURCE="$PWD/before/sustained.mjs" node --test --test-reporter=tap after/queue-selection.test.mjs
LUCI_SUSTAINED_TEST_SOURCE="$PWD/after/sustained.mjs" node --test --test-reporter=tap after/queue-selection.test.mjs
node --check after/sustained.mjs
```
