# Sustained attempt 4 — failed tool-response handling and incomplete acceptance

Product: **`2da1a821b8456c20a858fb86ef87649ea917d9de`**. Clean helper: **`abd2d8c9314d6b73d0c202c82de72f3c3acc0d9a`**. This was installed Linux Docker and the native Codex CLI with a **scripted local provider**, 24 provider calls and zero external model calls. The run failed. It does not establish live-model quality, full sustained four-project acceptance, Windows behavior, automatic cleanup or product capacity release.

The retained package assertion records 3,587 matching archive members. Exact archive, binary, web and runtime-image hashes are in `source-bindings.json` and the retained provenance files. All 16 accepted helpers and the separate sampler match immutable abd helper Git blobs byte-for-byte. The capsule preparation does not independently retrieve/rebuild the Linux package or image.

## Reached workflow and original failure

Four projects were prepared and queued. The recorded browser checks cover distinct unsent draft restoration, queued-item cancellation and priority changes. Exactly **one actual Docker Create and one Start** occurred. A real task issued its Go toolchain/test/build/fuzz commands. The per-command events are retained; they do not substitute for an accepted final task result.

At `active-command-restart`, the driver recorded an exact restart from **02:04:31.326 to 02:04:34.223 UTC**, retaining the same container, PID and context with one command dispatch. This is an observed active-command restart, not natural task completion or the rest of the failure/retry/integration workflow.

The original terminal failure is preserved:

> Actual tool output must supply completion receipt or live command identity

It originated in the scripted provider's response handling during `active-command-restart` (**CLOUD-041**). The full stdout contains the completion receipt, but the actual provider-facing tool output explicitly reports truncation and lacks both the receipt prefix and a live command ID. The exact effective truncation configuration is not identified by these records. No final task receipt was accepted: `actualTaskResults` is empty and `actualTaskCount` is zero. This does **not** mean zero workers. Both terminal attachments recorded zero native exit frames and `not-observed` natural exit. The original driver failure diagnostic says no settled native failure bundle was available. Later read-only examination recovered the actual command stdout and the truncated provider-facing tool output. Those exact retained forms, the printed completion receipt and the matching build artifact are included under `retained-output/`; the private full conversation/rollout is not. The command exited 0, the complete printed receipt records **150 active fuzz seconds and 6,846,700 executions**, and the output commit is `e6b673061966409e8805e56aad472ddd02672507`. These records independently support command-level success; the 188.374-second wall event alone would not. They do not rewrite the original empty accepted-task result, establish natural native-session exit, or certify corpus retention and cleanup.

Wrapper exit was **1** after **418.182829 seconds**, ending at 02:07:59 UTC. The driver ended at 02:07:44 UTC. The 30-minute workflow budget is a maximum, not a duration achieved. Other projects' task completions, reviewed integrations, amendment/replacement work, complete loaded sample quotas and settled calibration were not reached.

## Partial latency — original thresholds fail

Every one of the **696 recorded actions succeeded**: 400 idle actions and 296 partial-loaded actions. Original measurements run from automation command through expected DOM content and two animation frames, including transport and waits. They are not physical input-to-display latency. No row filtering or instrumentation-cost subtraction is applied.

| Phase | Action | Count | p95 ms | p99 ms | Original requirement |
| --- | --- | ---: | ---: | ---: | --- |
| Idle | Project switch | 100 | 385.298971 | 460.619650 | p95 ≤ 300 ms — fail |
| Idle | Input | 200 | 64.824527 | 132.009124 | p95 ≤ 100 ms — pass at this partial scope |
| Partial loaded | Project switch | 74 | 550.770057 | 1445.939623 | p95 ≤ 300 ms and p99 < 1000 ms — fail |
| Partial loaded | Input | 148 | 68.005099 | 321.629731 | p95 ≤ 100 ms — pass at this partial scope |

The 100 idle resets and 74 loaded resets remain in both the complete result and summary. Reset p95/p99 were 392.195168/519.004621 ms idle and 698.734558/2339.438521 ms loaded; no separate reset threshold is invented. Loaded switching/input totals remain below the required 200/400. There is no final successful latency-gate assertion.

`analysis/interactions-with-spans.projection.json.gz` retains **every row and every original numeric/reference diagnostic**, including node stages, clock reads, browser frames and limitations. The sanitized full result retains the original passive browser/event/network ledgers. `recompute-interactions.py` recomputes all six nearest-rank summaries offline. No action-only trace ZIP existed for this sustained attempt. Offline temporal correlations, if included under `analysis/`, do not establish React-function causality or a causal comparison with earlier runs.

The same-run quiet app-open phase lasted from 02:01:24.034 to 02:02:24.258 UTC, followed by idle calibration until 02:03:29.906 and a partial loaded interval. This is not a matched no-observer control or a completed sustained responsiveness/fairness test.

## Resource evidence scope

All original observer samples, metadata, summary, lifecycle and interpreter self-cost are retained with exact input bindings. The observer recorded **414 samples over 417.950321 seconds**. The raw sampler summary is not automatically a resource acceptance pass, causal explanation of a latency tail, or atomic admission measurement. The separately hash-bound offline analysis records **387 valid control-plane cgroup observations, with zero formula mismatches**. Minimum effective available memory was **10,243,493,888 bytes**, versus minimum physical availability **21,255,237,632 bytes**; these are separately scoped minima. It records 223 container-identity and 220 usage rows, with peak observed worker-cgroup memory **446,545,920 bytes**. Physical/effective availability, visible ancestors, unknown observations and overlapping process/container measurements remain distinct. Full sampler duration and the cgroup-reading section are separately reported. Sequential samples do not establish atomic admission values or a causal explanation for earlier refusals.

The observer interpreter used **81.071850 CPU seconds over 418.038526 wall seconds**, about **0.194 core**, with a peak RSS of **19,464,192 bytes**. This excludes spawned Docker CLI/helpers and daemon work. No subtraction of this measured overhead is used to change the latency result. This failed partial run cannot establish long-term memory growth, four-project throughput/fairness or comfortable Windows whole-computer use.

## Cleanup and retained charge — LOCAL-016 remains open

All five startup acquisitions closed, and controller/proxy/browser/provider cleanup phases completed. Proxy requests drained with no pending mutation uncertainty. The driver nevertheless reported:

> Unexpected full worker remained; exact failed-fixture cleanup required

The helper removed the exact owned failed-fixture worker after its ownership checks. Empty Docker inventory after that action is not natural task completion or product capacity release.

The independent read-only closure at **02:09:11.551839 UTC** found Docker containers/volumes empty. Among **16 recorded PID/start identities, 14 were absent and two Chromium identities remained exited zombies adopted by PID 1**. Three unrelated process environments were unreadable. No whole-machine zero-process assertion is made.

The original fixture ledger still retains exact lease **`queue-1b52e8733b6479438946f479`**, project **`p-b6e34e18`**, session **`s-a692964a`**, in **`backend-settled`** state, charged **1 GiB / 1,000 mCPU**. Ledger SHA-256 is **`a5f9d4a08b6ca56e726d6ce88ad1cf51bd489b39b1eff66de8a8194bfda10d82`**. Product lease release and automatic cleanup remain unpassed. This is linked to the existing LOCAL-016 retained-capacity scope; it does not establish a new product root cause or justify deleting the charge.

Root's closure recorded **375 private fixture files unchanged**, with all original artifacts unchanged. Private inventory rows and original fixture contents remain external. The published closure is an allowlisted projection bound to the exact original SHA; it is not the original private receipt. Publication performs no recovery, ledger edit, deletion or signal against the failed fixture.

## Publication, privacy and reproduction

`publication.json` binds each original artifact, every included member, decoded gzip bytes, exact helper sources and declared sanitization. Full final result text is preserved except explicitly listed private-path substitutions; its compressed publication must not be described as exact original raw bytes. Where no substitution occurs, decoded gzip bytes match the original exactly. No numerical values, original failure wording or success/failure flags are corrected. The incomplete receipt, journal, wrappers and samples remain separately preserved. The screenshot was visually inspected and contains only synthetic acceptance UI.

Known private roots and owned-fixture retained password/session hashes were checked in memory against recursive JSON strings, decoded gzip/text and plausible base64 values. Credentials, account/session databases, private inventory rows and original fixture source/state are excluded. The scan does not guarantee absence of every possible secret. Synthetic project/operation/session IDs and runtime measurements remain where needed to follow exact ownership and evidence.

Reproduction requires the exact package/image and clean helper source, a **fresh** isolated output/state directory and an exclusive disposable Linux Docker daemon. Use the retained invocation and pinned `scripts/acceptance/sustained-four-project/README.md`; do not reuse the failed private fixture or run the exclusive-daemon harness against personal shared workloads. Preserve the 6 GiB Create floor, 1 GiB working reserve, 1 GiB/1 CPU worker limits, original deadlines, task workload and latency targets. A corrected helper rerun and the independently open performance/recovery/Windows/live-provider gates are still required. This capsule was prepared offline outside the checkout, without launching an app, worker, compiler or test suite.
