# LOCAL-023 controlled latency helper validation

One authorized invocation of `scripts/acceptance/sustained-four-project/latency-diagnostics.test.mjs` completed with **17 passed, 0 failed, exit 0**. There were no skipped, cancelled, or todo tests. Raw TAP reports 149.744536 ms; the process wrapper observed 0.219671 seconds. Stderr is empty. No rerun was performed.

The actual checkout was `/workspace/luci`, HEAD `6f00501d7f370789ed45b018a7d958fd25cc1e2d` plus the existing root diff. The test used the actual checkout files. This harness made no repository edits and started no browser, Docker, or service. These controlled VM/fake-transport tests do not establish browser latency, physical painting, real-model quality, or native application acceptance.

Invocation, after `. /workspace/.luci-tools/activate.sh`:

```sh
node --test --test-concurrency=1 --test-reporter=tap /workspace/luci/scripts/acceptance/sustained-four-project/latency-diagnostics.test.mjs
```

- Start: `2026-10-11T10:25:14.557708+00:00`.
- End: `2026-10-11T10:25:14.777618+00:00`.
- Runtime: Node `v24.19.0`, `/opt/codex/runtimes/codex-primary-runtime/dependencies/node/bin/node`.
- Node executable SHA-256: `bc17c508ffeed0ec622934f9b7fa72f8e78da65350e63c3eceb56fa688aa5e12`.
- Root `git diff HEAD --binary --no-ext-diff` SHA-256: `f178a7c1842bafb658424feae00ccd96215d5a452a66701ca29f6e772cea18fa` (53,659 bytes).
- Execution bound: 115 seconds, then owned process-group termination with a 3-second grace. No timeout or cleanup signal was needed.

`before/receipt.json` and `after/receipt.json` record matching source/configuration hashes, matching Node executable identity, unchanged HEAD/root diff/git status, complete discovered import edges, and searched ancestor package/runtime configuration names. Copies of the test, both local transitive imports (`latency-diagnostics.mjs`, `interaction-metrics.mjs`), current root `AGENTS.md`, and activation script accompany both receipts. All other imports are Node built-ins. No searched ancestor package/runtime configuration file was present; desktop/Vitest configuration was not selected. `NODE_OPTIONS` and `NODE_V8_COVERAGE` were absent; `NODE_PATH` was present. Environment values are not copied.

`invocation.json` records the argv, working directory, start time, PID, and bound. `result.json` records final counts, process exit, timing, and comparisons. `stdout.tap` and `stderr.txt` retain raw process streams. `run-once.py` is the capture wrapper; `RUN_STARTED` prevents it from executing a second time. The files under `before/` and `after/` are external evidence copies, not execution inputs.

Verify the artifact files without rerunning the test:

```sh
cd /workspace/.luci-tools/local023-validation-20261011/helper-latency
sha256sum --check SHA256SUMS
```

The checksum manifest covers every retained regular file other than itself.
