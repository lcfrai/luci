# Primary markers diagnostic 1 — completed capture, invalid CPU timing, no acceptance claim

This run used packaged product **`2da1a821b8456c20a858fb86ef87649ea917d9de`**, clean checkout guard **`e2f426bb50267a7cceac1f53eb1e00a0a4cc278e`**, and separately frozen runtime helpers originally accepted with **2da**. The external diagnostic candidate is measurement code, not an unchanged acceptance collector or product-source correction.

The driver and wrapper completed successfully. All **400 primary actions** succeeded: 100 switches, 200 input actions and 100 resets. Marker trace transport completed without reported data loss, within the original bounds. **That completion does not establish valid CPU time attribution, a passed performance requirement or full local acceptance.** The later corrected analyzer rejects this run's CPU duration weights because the raw profile contains negative deltas.

| Instrumented action | Count | p95 ms | p99 ms | Maximum ms |
| --- | ---: | ---: | ---: | ---: |
| Project switch | 100 | 349.842204 | 504.425700 | 557.839127 |
| Idea input | 200 | 78.436254 | 140.021498 | 208.607676 |
| Draft reset | 100 | 347.844812 | 458.246545 | 549.854179 |

The original acceptance targets remain switch p95 ≤300 ms, input p95 ≤100 ms and p99 <1000 ms. This diagnostic includes profiling/passive-marker overhead and cannot replace that acceptance run, establish a causal gain or be corrected by subtracting overhead. Its switch p95 also exceeds the original reference. All rows and original numeric diagnostic spans are preserved; none is filtered. The ordinary 60-second quiet period and 400-action sequence remain, and no extra 32-action followup ran (`trace-after=no`). `recompute-quantiles.py` recomputes all three nearest-rank groups from the public projection without app/runtime work.

## Capture and analysis scopes

The narrowing changed only `primary-profiler.mjs` and `latency-diagnostics.mjs` from the preceding prepared candidate. Broad timeline/V8/GC categories were replaced by **`blink.user_timing` only**, with the existing whole-window CPU profile retained. The unchanged bounds are 150-second recording allowance, 5 ms requested CPU interval, 16 MiB `recordUntilFull` trace buffer, 32 MiB retained timeline, 12 MiB serialized CPU, 256 read chunks, 9-second internal/10-second outer finish, 210-second workflow and 450-second wrapper. No limit was increased to obtain this result.

The profiler records `complete: true`, `traceDataLoss: false`, no failures, no timer expiry, zero pending commands and successful detach. Marker buffer peak was approximately 0.058894. Recorded artifacts are:

- Marker timeline: **1,032,413 bytes**, SHA-256 `c0de6a0d19f2f14b07b25f882465e27e6039a5c556ed9514aeb585c147bec573`.
- CPU profile: **701,428 bytes**, SHA-256 `5f925f41f6b9a080f590d98a5168786ccbbc7d2d03da0eb0934941f154e89f79`.

Both hashes were rechecked during publication; the timeline was not parsed during publication and neither raw payload is included. The existing allowlisted analysis records **1,200 markers: exactly three for each ordinal 1–400**, on one renderer thread. Its 800 browser/trace offset anchors have approximately 0.565 ms spread, which is exposed rather than assumed zero. Marker coverage is complete for this selected category; it is not a broad rendering trace.

Passive long-task and long-animation-frame APIs were supported in all 109 captured windows. Those APIs omit shorter events, and the at-most-64 script summaries per long frame disclose truncation. Overlapping durations must not be summed. Style/layout/paint and other omitted trace categories are **not captured**, never measured zero. The switch's post-dispatch browser witness starts at the final project-open pointer; it excludes the preceding overview click and any earlier transport/actionability wait. The expected-DOM marker is the driver's locator observation, not the first app-ready or physical paint instant. Node stages include transport and scheduling. No individual React component or backend function is identified as the cause of an action tail.

## Corrected CPU analysis supersedes the original weights

**All weighted CPU, frame or bundle attribution is withheld.** The exact profile contains 14,755 samples and deltas, including **1,288 negative deltas totaling −3,219,079 µs**, with minimum −8,739 µs. The signed sum is 63,567,291 µs versus stored endpoint span 63,569,417 µs. Equal counts and a nearly matching total do not validate monotonic duration allocation. No values were clamped, dropped, sorted, absolutized or replaced with the requested interval. The cause of the negative values is not established.

`analysis/allowlisted-analysis.json` and `analysis/analyze-before-negative-delta-check.py` preserve the **superseded preliminary analysis and its old source**. Their weighted bundle values must not be used. `analysis/allowlisted-analysis-v3.json`, SHA-256 `10a1d1055f4c961b41804bffdaa2aab1a06f8aa9f8b5364fce8c4f2d0e1bc00f`, and the corrected `analysis/analyze.py` explicitly reject weighted attribution and retain `topBundleLocations: null`. `shape-and-stage-audit.json` independently records the profile-shape issue. Publication reread only CPU timing/identity arrays to verify these counts; it did not rerun either analyzer, repair the profile or reopen the timeline's event contents.

The eight recorded corrected-analyzer cases include six earlier cases, so these counts overlap. The correction adds negative-delta and unknown-sample-identity refusal coverage. These are controlled historical checks, not runtime performance evidence. Original and corrected files, empty stdout/stderr files and source bindings are preserved separately. No new test invocation occurred during publication.

## Executed source and historical preparation

`sources/accepted/` preserves all 17 exact local source snapshots, gzip-compressed and matched to their accepted run copies. The binding separately retains seven frozen helper dependencies and 73 installed Playwright/playwright-core tooling hashes. Tooling hashes are preparation/run records and were not rehashed during publication. The clean checkout identity does not turn external diagnostic modifications or installed tooling into product source. The frozen task command remains the earlier pre-CLOUD-041 version; no worker/provider task ran here.

The candidate binding is `6efa2bffae8fd8022d3179b10cca14286788202a0c9fafa451e4576cd64999d9`; launcher hash is `ad58897ea57eb2660823ef344276b7dc960bc2b866dedeb66d195db049427494`. Immutable invocation, source and package/image receipts tie the run to its inputs. The 3,587-member package assertion remains the original publisher's verification, not a new retrieval/build.

`preparation/README.pre-run.md` and readiness/validation records are historical. Statements that the candidate was not yet run and any old unqualified CPU-weight wording are superseded by this run's receipts and the correction above. Preserved controlled fixture/syntax results validate their recorded input set only; do not add overlapping case counts or treat old-source checks as a final corrected-analyzer suite. This capsule includes the actual accepted source, not an unexecuted intermediate candidate. The first failed CDP1 capture remains separately unchanged.

## Cleanup and privacy

The wrapper records exit **0**, no external-monitor timeout and **147.538812 seconds**, from 02:54:47 to 02:57:14 UTC. Original driver cleanup records no errors, closed handles, drained/certain Docker state and removed fixture; it records zero Create/Start, tasks, provider calls and external model calls.

The independent closure at **02:58:39.606295 UTC** verifies the known driver PID/start identity absent and the recorded controller PID absent, empty exact fixture parent, and empty Docker container/volume inventories. **Browser identities were not independently captured.** Three unrelated process environments were unreadable. There is no whole-machine, zero-zombie or independent browser-identity-closure claim. A narrow closure projection omits private location fields; its original source hash remains bound.

Raw report, marker timeline and CPU payloads remain private because arbitrary event arguments, source URLs and API contents may contain identifying data. The public report is an allowlisted projection, never exact raw bytes. It retains selected receipt fields and all 400 unfiltered timing rows/diagnostics. Exact private fixture paths are removed from invocation/metadata/log representations; generic preparation/source literals are retained only as code provenance. The known-root scan and allowlist are not a universal secret-scan guarantee. No private fixture credentials, request/response bodies or raw profiler event arguments are published.

## Reproduction

Use a fresh output name and short private parent with the exact package/image provenance, clean helper checkout and accepted diagnostic source/tooling bindings. The sanitized invocation retains exact argument values and explicit private-location placeholders. The original launcher accepts `--helper-source e2f426bb50267a7cceac1f53eb1e00a0a4cc278e --run-name <FRESH_NAME> --execute` and generates a fresh 0700 parent after checking both socket paths. Preserve the original exclusive disposable Docker contract and hard-zero Create/Start guards; do not run this exclusive harness against personal shared workloads. Do not reuse a retained fixture or overwrite a prior run.

For later analysis, use the corrected analyzer with the exact private receipt/profile directory and a new output path. The preliminary analyzer is retained for audit only and must not supply trusted weighted CPU output. Raw private payloads are deliberately absent from this public capsule, so their later analysis requires separately authorized access to the original evidence. A successfully completed diagnostic still leaves unchanged acceptance latency, sustained multi-project resources, native Windows, real-provider quality and full local acceptance open.
