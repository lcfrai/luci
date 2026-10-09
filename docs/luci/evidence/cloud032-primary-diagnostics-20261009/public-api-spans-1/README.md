# Public API span diagnostic 1 — failed logger coverage

This is one separately instrumented diagnostic, not an acceptance run. Product/package source is `2da1a821b8456c20a858fb86ef87649ea917d9de`; the clean helper-checkout guard is `765fcaadeee7de1cf2ba3a3fb7b295d0af445b70`. Runtime dependency copies remain frozen from 2da, while the executed external diagnostic candidate adds marker profiling and the public API logger. These identities are deliberately separate.

All 400 original actions completed: 100 project switches, 200 input actions and 100 draft resets. Instrumented p95 values were 363.614553 ms, 64.931624 ms and 418.412612 ms respectively. Switching p99 was 516.708111 ms. Every action and original numeric diagnostic span is retained in `result.projection.json.gz`; nearest-rank quantiles are independently recomputed. These values include instrumentation and cannot substitute for acceptance, establish improvement or be adjusted by subtracting overhead. The unchanged 300/100 ms acceptance targets are neither relaxed nor asserted to pass here.

## Original failure and what remains unknown

The run failed at `idle-baseline` with `Public Logger coverage incomplete; no retry-absence or duration attribution claim`. Wrapper exit was 1 after 150.078310 seconds, with no external-monitor timeout or wrapper signals.

The public logger recorded only 137 allowlisted callback entries over 400 action records. It reached the configured 100-problem bound; all retained problem codes were `unrecognized-api-message`, and `overflow=true`. The retained entry body was 12,293 bytes, below the 2 MiB byte cap. The exact original entry stream, action records, limits and problem codes are preserved separately. No successful full call-sequence analysis is manufactured from this incomplete stream.

The unknown callback names/messages were not retained, so their exact historical source cannot be recovered from this receipt. A later controlled smoke reproduced a possible mechanism involving background public API callbacks; it does not prove that those callbacks were the original 100 messages. That correction and its tests belong to another capsule. This capsule contains the original failed helper only.

## Complete CDP transport does not validate CPU timing

The separate CDP marker capture completed, with `traceDataLoss=false` and no profiler failures. The raw timeline was 1,031,721 bytes, SHA-256 `1ed3a1d81089e82fcb4ffd3696fc68d3bc8a03fd66aad80fe38a0d5fba64b09c`. The raw CPU profile was 672,914 bytes, SHA-256 `9c5fb262a8e453c33d0436122391e2d48f5ffe26ff8386183a54a175f906c170`. Both remain private because arbitrary trace arguments, URLs and frame names are outside the publication allowlist.

The corrected v3 analyzer reread and bound these exact originals without running the app. It found 14,990 CPU samples with 1,297 negative deltas totaling −3,322,980 µs; minimum delta −50,169 µs. Weighted CPU, exclusive/inclusive bundle or frame attribution, percentages and per-action CPU attribution are therefore withheld. No negative values were clamped, removed, reordered or replaced by the nominal sampling interval. The cause of these deltas is not established. `analysis/allowlisted-analysis-v3.json` exports `topBundleLocations=null` and records the rejection explicitly.

There are 800 paired browser-marker anchors for the 400 actions; their offset spread is 0.438 ms. That is a browser/trace witness, not permission to subtract Node and browser clocks. Marker capture excludes broad style/layout/paint/GC categories: absence is not zero work. Expected-DOM markers reflect the driver's observation, not the first renderer-ready or physical-paint instant. Passive long-task/frame observations are thresholded and incomplete for shorter work. No Playwright tracing was enabled; CDP tracing **was** enabled. This was not a logger-only run.

Original capture bounds: 150-second recording allowance, 5 ms requested CPU interval, 16 MiB trace buffer, 32 MiB retained timeline, 12 MiB CPU file, 256 read chunks of at most 256 KiB, 9-second internal finish, 10-second outer finish, 210-second workflow and 450-second wrapper. Logger limits were 400 actions, 8,000 entries, 2 MiB entry bytes and 100 problem codes. Original bounds and overflow observations remain unchanged.

## Sources, closure and reproduction

`sources/accepted/` contains 18 compressed, byte-exact accepted source files, matched against the frozen candidate. Seven runtime dependencies and 73 installed Playwright/tooling inputs are hash-bound separately; installed tooling was not rebuilt or rehashed during publication. `invocation.sanitized.json` preserves the exact argument sequence with explicit private-location placeholders. Package/image integrity is the original run's observation, not a fresh artifact download.

The original preparation README still says no runtime had launched; it is preserved as historical preparation text. Its initial nine pure tests overlap the later ten, not 19 unique checks. They establish only the declared helper cases and did not prevent the real callback-coverage failure. No helper/app tests or runtime were rerun for this publication.

The independent closure at 2026-10-09T03:12:48.330675Z observed the exact recorded driver PID/start identity absent and the recorded controller PID absent. Browser PID/start identities were not independently captured, and three unrelated process environments were unreadable; there is no browser-zombie or whole-machine zero-process claim. Docker containers and volumes were empty. The closure records 238 private fixture files and original inputs unchanged, with no signals or mutations. The fixture remains retained. No ledger observation is invented where the closure did not record one. Driver cleanup and five startup-handle closures remain their separately retained observations.

To reproduce the **original failing diagnostic**, first provision the exact package/image and pinned tooling recorded in the provenance on a disposable Linux host with exclusive empty Docker. Restore the accepted gzip sources to the recorded candidate directory, retaining the recorded external source binding and source hashes. The preserved launcher expects the original preparation/package layout; adapt environment paths explicitly, recording a new binding, if it is relocated. Use a new run name and fresh state; never reuse the retained failed fixture or run this exclusive harness against personal shared Docker workloads.

```bash
source /workspace/.luci-tools/activate.sh
python3 /workspace/.luci-tools/cloud032-public-api-logger-preparation/launch.py \
  --helper-source 765fcaadeee7de1cf2ba3a3fb7b295d0af445b70 \
  --run-name public-api-spans-reproduction --execute
```

The original launcher and helper-checkout guard require a clean matching checkout. This command is reproducibility documentation, not evidence of another execution. The separate `preparation/analyze-api-spans.py` intentionally refuses incomplete logger coverage and was not run to create a passing analysis here. Offline public quantiles can be checked with `python3 recompute-quantiles.py`. For private raw profile validation, the exact executed analyzer and its input bindings are retained; its invocation requires `--receipt`, `--profile-dir` and a new `--output` and writes only allowlisted results.

Publication is external, read-only analysis and source/metadata copying. No repository edits, browser, application, compiler or workload were launched. Raw private parameters/messages, CPU/timeline payloads and fixture inventories are excluded. Known private-root checks are bounded and are not a universal secret-scan guarantee.
