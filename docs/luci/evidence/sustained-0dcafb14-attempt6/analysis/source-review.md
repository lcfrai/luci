# Sustained attempt 6: failed workflow, complete observed cohorts

The exact product and acceptance helper are both `0dcafb14da7d1f7e2eedaf70366f1e3cf4f4303f`. The original result SHA-256 is `9b26b0f3fff80e37f61b07dbf6a740b38c2e1f52cff16f6466104904e140feea` (23,445,411 bytes). This is retrospective analysis of one failed installed Linux/Docker/native-Codex run with a SCRIPTED provider. It is not a live-model, Windows or completed local acceptance result.

## Reached workflow and original failure

Build A, C and D produced three accepted command receipts. Each native session emitted exit-status 0, reached exact stopped/absent container observation, and then reached queue completion with its own lease absent. The recorded lease disappearance times are A 03:46:37.038Z, C 03:51:08.557Z and D 03:55:38.547Z. Each records one coding dispatch and one final response, with no final-response abort/retry. These observations remain bound to the exact task/session/operation records.

Two reviewed integrations completed, A and C. D remained unintegrated when the workflow waited for the remaining B allocation. The recorded stage remains `actual-work-build-d`; the original error is `Exactly one queued worker allocated did not complete within its bounded prerequisite window`, from `queuedChild`. The stage name is not proof that D failed its command. B was not observed allocated and no B command result was accepted. This publication does not infer why B failed to advance, rewrite the driver result, or conflate later investigation with the original retained observation.

The run records 69 scripted-provider calls and zero external model calls. Source edits were driven by the deterministic acceptance workload; command/output quality observations do not establish live-model planning quality. The approved-context handoff checks, queue cancellation/priority observations, active-command restart and all allocation-order/progress records are retained in the exact receipt and focused proof projection. None is expanded into an unobserved four-project completion, final verification, reviewed amendment or settled idle phase.

## All latency observations, without adjustment

All 1,292 actions succeeded and remain included. These comprise 400 idle and 892 loaded actions. The loaded count quotas of at least 200 switches and 400 inputs were reached; workflow completion and the settled phase were not.

| Cohort / action | Count | p95 ms | p99 ms | Maximum ms |
| --- | ---: | ---: | ---: | ---: |
| Idle switches | 100 | 485.371075 | 660.574852 | 775.674473 |
| Idle inputs | 200 | 64.613646 | 119.201686 | 774.099349 |
| Idle draft resets | 100 | 501.310223 | 600.517997 | 667.089320 |
| Loaded switches | 223 | 551.556830 | 973.380024 | 1351.242503 |
| Loaded inputs | 446 | 72.251546 | 258.327290 | 426.715775 |
| Loaded draft resets | 223 | 504.306572 | 688.523001 | 1019.484394 |

Every duration and diagnostic span was retained; quantiles use the declared nearest-rank rule. The idle and loaded switch p95 values both fail the unchanged 300 ms target. Both input p95 values are below 100 ms, and the listed cohort p99 values are below 1,000 ms. Maximums above 1,000 ms remain visible; maximum and p99 are not the same statistic. This is still not a passing full performance gate. No observer cost was subtracted, no failed threshold was relaxed and no physical input-to-display or causal speedup claim is made.

Eleven idle and 88 loaded switches exceeded 300 ms. Representative original tails show different stages:

- Idle row 132 totals 775.674473 ms and includes a 445.479490 ms overview-heading wait. Its matched All projects click handler was 31.9 ms, with 456 ms thresholded EventTiming duration.
- Loaded row 1040 totals 1351.242503 ms, including a 1138.469961 ms Open-project click call. Its matched pointer event reports 866.2 ms queue delay; the matched click reports 885.1 ms queue delay, 87.9 ms handler and 1088 ms EventTiming duration. This is an observed browser-side event queue delay, not an attribution to a specific component or host cause.
- Loaded row 700 totals 1309.718271 ms, including a 1092.503615 ms All projects click call; its matched click handler is 58.8 ms with 152 ms EventTiming duration. A large interval lies outside that handler, but its origin is not established.
- Loaded row 424 totals 973.380024 ms, including a 595.419929 ms overview-heading wait, a different stage from the two preceding tails.

Node call spans include transport, actionability, scheduling and response waits. Passive EventTiming matching is temporal with the recorded quantization tolerance; missing thresholded events are not zero. No primary actionability or CPU trace for these exact actions is present, so separate diagnostic runs cannot be retroactively mapped onto these tails. No specific React component, backend function, GC or host scheduling cause is assigned.

## Resource evidence and its limits

The complete observer stream contains 1,132 samples: 1,107 observed control-plane cgroup records, 24 not observed and one partial. All 1,107 valid effective-memory formulas recompute with zero mismatches. Minimum observed effective available memory is 7,089,815,552 bytes; minimum physical available memory is 20,244,770,816 bytes. These different scopes are not interchangeable. Approximately one-second sequential samples and nearest brackets are not atomic Create/Start admission values or evidence of the allocation refusal's cause.

Sampled owned-process RSS sum peaks at 1,479,077,888 bytes. Browser-role peak is 1,112,662,016 bytes; control-plane peak is 174,948,352 bytes. Per-role maxima need not coincide, summed RSS can double-count shared pages, and container cgroup usage must not be blindly added to overlapping host scopes. The three exact containers have identity/usage coverage A 246/215 rows (peak 455,655,424 bytes), C 236/218 (425,832,448 bytes), and D 236/221 (427,290,624 bytes).

The observer interpreter consumed approximately 0.195688 core over 1,135.967893 seconds. That self-cost excludes spawned Docker CLI/helper work and all Docker daemon work. No matched no-observer run or causal overhead correction is available. There is no complete settled-memory-growth, four-project-throughput, Windows or whole-computer responsiveness result.

## Independent closure

The read-only closure at 2026-10-09T04:03:51.505374Z records an empty fixture execution ledger (SHA-256 `c0022f0995797c072d8097729ea90e74487bcefb911576c91e4af0665403a49e`) and empty Docker container/volume inventories. Unlike earlier failed attempts, this run has no retained execution charge. It does not repair or close the earlier retained fixtures.

Of 15 recorded PID/start identities, 13 are absent and two Chromium zombies remain adopted by PID 1. Three unrelated process environments are unreadable. This is not whole-machine zero-process evidence. All 703 recorded private fixture files and original run artifacts remained unchanged, with no signals or mutations during the independent closure. Private original ledger/inventory bytes remain outside the public capsule; the public projection binds the root's observation and must not be described as independently retrieved original private bytes.

The original failed result, incomplete checkpoint, journal, screenshot, wrapper/observer reports, exact helper snapshots, package/image provenance and full timing/resource inputs remain separate and unchanged. Successful closure observations do not turn the unfinished workflow into acceptance success.
