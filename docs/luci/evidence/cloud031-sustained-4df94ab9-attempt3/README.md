# Sustained attempt 3 — failed parser and incomplete performance acceptance

Product source: **`4df94ab9dad0e9fedd28a873bbadc56720e1402e`**. Clean helper source: **`5d306b6dae3cc9d0a4d9cbf95f8142c37958964f`**. This was an actual installed Linux Docker/native Codex run using a **scripted provider**, with 24 local provider requests and **zero external model calls**. It failed; this capsule does not establish sustained four-project acceptance, live-model quality, Windows behavior or full cleanup.

The retained package provenance identifies archive SHA-256 `0ac05247a194dd3cd8851c2111b1eda5906b399dbb892438484ff2b508c80d1d`, binary SHA-256 `6d6b11dd4382db11161384b23a55f1596e948f68c5a049a7e0aa7e409c724a9c`, and 2,021 archive-matched members. Runtime image is `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`. These are retained local build/run assertions, not evidence that a separate tester retrieved those binary artifacts. All 16 accepted helper files and the sampler match the pinned helper commit byte-for-byte.

## Reached workflow and failure

Four separate projects were prepared and queued. The recorded checks passed project-scoped unsent-draft restoration, exact queued-item cancellation and priority changes through the browser. One worker was actually created and started. Its intentionally failing requirement test detected the missing implementation; after the scripted edit, build, fixed tests and artifact behavior passed. The fuzz subprocess exited zero.

A controller restart during that real active fuzz command, `21:46:55.703Z`–`21:46:58.547Z`, retained the same exact container, worker PID and context, with one command dispatch. This is the scope of the recorded restart check. The terminal observer did **not** observe a natural session exit, and the full task receipt was not accepted. It is not proof of the remaining failure/retry/integration workflows.

**CLOUD-031 is a helper parsing failure.** The [original decoded fuzz log](result/decoded-workload-logs/fuzz.log), SHA-256 `506a10848d01b2bb297279b64634aef1bf641c3dcef52080f5683c8cf99161aa`, ends with `elapsed: 2m30s`, **6,993,018 executions**, `PASS`, and a package duration of **150.078 seconds**. The seconds-only parser stopped at its last matching line, `57s` / 2,560,985 executions, then rejected the required 150-second workload. The raw failed receipt preserves those incorrect parsed values; they have not been rewritten. The subprocess took 182.028 seconds of wall time including overhead; wall time is not substituted for active fuzz time.

The parent driver caught the failed helper output at `active-command-restart`, `21:49:49.870Z`, before any completed task receipt or integration. Its `actualTaskCount: 0` means no accepted final task results, **not** zero workers. The Docker transport recorded one Create (201) and one Start (204). The other initial projects, amendment, final settled calibration and complete throughput/fairness workload were not reached. Wrapper exit was **1**, after **440.944 seconds**. The 30-minute scenario value was a maximum deadline, not a completed duration or minimum workload claim.

## Partial interaction measurements — targets not met

These are end-to-end automation measurements: command dispatch, expected DOM contents and two animation frames. They include browser transport and waits; they are **not physical input-to-display latency**. All recorded actions succeeded, but completion and latency gates did not pass.

| Phase | Action | Samples | p50 ms | p95 ms | p99 ms | Maximum ms |
|---|---|---:|---:|---:|---:|---:|
| Idle calibration | Project switch | 100 | 256.522 | 523.941 | 713.255 | 747.853 |
| Idle calibration | Input | 200 | 52.567 | 120.045 | 250.750 | 341.950 |
| Partial loaded phase | Project switch | 74 | 294.365 | 492.327 | 621.145 | 621.145 |
| Partial loaded phase | Input | 148 | 55.038 | 82.602 | 167.048 | 219.345 |

Predeclared targets remain project-switch p95 **≤300 ms**, input p95 **≤100 ms**, and p99 **<1,000 ms**. Both switch groups and idle input exceeded their p95 targets. Loaded samples fell short of the required 200 switches/400 inputs; settled calibration was never reached. The raw result does not contain a successful final `latencyTargetsMet` assertion. The extracted [interaction summary](analysis/interaction-summary.json) also retains draft-reset measurements. Original per-action spans, passive timing windows and diagnostics are in the complete compressed result. The passive ledger records 31,023 events, 6,115,273 accounted bytes and verified sequence ordering across 13 documents; that is buffer/sequence evidence, not a performance pass.

There was a 60.281-second quiet app-open baseline before the 82.337-second idle calibration, followed by a partial loaded phase. This is useful same-run comparison, but is not a no-observer control, long-term memory-growth test, human-speed stress test or whole-computer responsiveness measurement.

## Sampled resources and cgroup visibility

The observer recorded **440 samples over 440.635 seconds**. Of those, **411** contain a successfully observed exact control-plane PID/start identity and visible cgroup hierarchy; **29** explicitly lack that observation. Both pre- and post-restart control-plane identities are retained. The partial loaded interval contains 249 samples, 248 observed, with one observation gap at restart. [Resource analysis](analysis/resource-analysis.json) binds the original input hashes and records phase boundaries and nearest event brackets.

| Recorded interval | Samples / valid cgroup observations | Minimum effective available bytes | Minimum physical available bytes |
|---|---:|---:|---:|
| Quiet app-open idle | 61 / 61 | 15,859,236,864 | 21,400,096,768 |
| Idle interaction calibration | 82 / 82 | 15,893,925,888 | 21,435,797,504 |
| Partial loaded phase | 249 / 248 | 9,551,831,040 | 20,970,274,816 |

The sampler's product-comparable effective value applies the visible cgroup limit-minus-current minima as well as physical availability. The observed effective total was limited to 32 GiB. Physical and effective values describe different scopes and are not interchangeable. At Create acknowledgement/Start, bracketing effective availability was approximately 10.27→10.17 GB while physical availability was approximately 21.54→21.52 GB. These approximately 1 Hz sequential samples are **not an atomic admission snapshot or proof of any prior refusal's cause**. Hidden ancestors are not inferred. Shared cgroup usage includes unrelated work, RAM-backed retained files and observation work; it is not memory attributed solely to Luci.

Peak sampled summed owned-process RSS was **1,428,213,760 bytes**, including the test driver and browser; it double-counts shared pages. Separately, the maximum observed worker cgroup memory was **459,165,696 bytes**. Do not blindly add overlapping process and container measurements. Docker daemon/shared kernel costs are not allocated to Luci by this receipt.

The observer interpreter itself used **80.199 CPU seconds over 440.749 wall seconds** (about 0.182 core on average), with peak RSS **19,267,584 bytes**. This excludes spawned Docker CLI/helpers and daemon work. The additive cgroup-reading section alone had p95 duration about **1.198 ms**, maximum **5.023 ms**; do not attribute the whole observer cost to that section. There is no matched run without the observer, so causal measurement overhead remains unquantified. All raw samples and self-cost evidence are retained.

## Cleanup: handles closed, retained capacity charge

Controller, proxy, browser and provider cleanup phases completed; all five startup acquisitions were closed. Proxy requests drained with no pending mutation uncertainty. **This is not a complete automatic cleanup certificate.** The driver recorded:

> Unexpected full worker remained; exact failed-fixture cleanup required

The helper then checked the exact created worker's ownership and stopped/removed that allocation during failed-fixture cleanup. Its later `runtimeCleanup` zero-container/volume observations cannot be presented as natural task completion or product capacity release.

The [independent read-only closure](independent-closure.json), recorded at `21:58:33.343032Z`, observed zero Docker containers and volumes. Of 14 previously sampled PID/start identities, two Chromium identities remained as exited zombies adopted by PID 1; no active sampled process or unknown identity check remained. Sampling cannot prove absence of never-observed descendants, and “zero owned processes” must not replace those literal observations.

The exact fixture execution ledger **still contains one charged `backend-settled` queue lease**, `queue-5e00d2dff81ac9d3cc2191bd`, for project `p-c209f019` / session `s-fac80621`: **1 GiB and 1,000 mCPU**. Ledger SHA-256 is `3698bc374c3b1f7754056fb9bfe419afd8c4f685289f4f121652a3ceac12d460`. Therefore `productLeaseReleaseVerified` and `automaticCleanupCertified` remain **false**. Independent inspection and publication performed no ledger repair, removal or recovery against the original failed fixture.

## Publication and reproduction limits

`publication.json` hashes every other capsule file, identifies exact original paths/bytes/hashes, and records gzip decoding and derived-log operations. The 12.4 MB final result, incomplete checkpoint, full diagnostic journal, samples and bounded workload failure receipt are preserved. The latter was explicitly reviewed: it contains 79 synthetic Go fuzz corpus entries and six workload logs. Decoded log exports are byte-identical to those base64 members; original incorrect parser fields remain intact. The screenshot contains only the synthetic acceptance UI. Directory-local attributes preserve captured bytes.

Generated fixture home/configuration, credentials, tokens, account/session databases and original source/state contents are excluded. Only the independently scoped ledger metadata embedded in the closure is published. Paths identify retained host evidence, not remotely retrievable artifacts. Do not reuse the private failed fixture. No workload, browser, Docker command, compiler or test suite was rerun to prepare this publication.

Reproduction requires the exact pinned Linux package, image and helper source and a **fresh** isolated fixture/output directory, using the command and unchanged bounds in `invocation.json` and the pinned `scripts/acceptance/sustained-four-project/README.md`. Preserve exclusive-daemon, ownership, 6 GiB pre-Create disk admission, 1 GiB working reserve, and 1 GiB/1 CPU/256 PID worker limits. CLOUD-031 requires a corrected-helper rerun; current latency misses and incomplete four-project/recovery/resource/live-model/Windows gates remain open independently of that helper correction.
