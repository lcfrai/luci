# Idle 972d: retained timing review

The original 100 primary project switches have p50 **204.949 ms**, p95 **400.975 ms**, p99 **513.632 ms**, maximum **530.788 ms**. Nine exceed 300 ms, so the original project-switch reference remains unmet. Primary input p95 is **79.123 ms**. All 432 actions completed: the original 400-action cohort and separate 32-action follow-up remain separate.

The largest recorded Node span among the nine slow switches is the **All projects command in six** and **expected-context verification in three**. This makes returning Home the strongest measured region for the next bounded investigation; it does not identify a product function or justify a particular code change.

| Primary switch ordinal | Original elapsed ms | All projects command ms | Verify context ms | Associated Home click processing interval ms | Destination pointer → driver DOM witness ms |
|---:|---:|---:|---:|---:|---:|
| 1 | 318.168 | 116.854 | 49.805 | Unavailable | 70.800 |
| 4 | 400.975 | 203.936 | 43.847 | 110.200 | 72.400 |
| 7 | 413.676 | 53.723 | 261.223 | 10.600 | 273.600 |
| 21 | 513.632 | 349.925 | 45.965 | 61.900 | 61.100 |
| 41 | 400.143 | 61.605 | 241.484 | 10.600 | 214.300 |
| 48 | 307.008 | 121.343 | 47.106 | 19.600 | 73.600 |
| 63 | 530.788 | 350.726 | 43.999 | 57.500 | 58.100 |
| 71 | 504.865 | 269.015 | 62.524 | 13.400 | 107.400 |
| 74 | 504.048 | 60.937 | 346.414 | 11.900 | 336.000 |

**Before versus after the actual Home pointer is unknown for the whole command.** The retained Node command boundaries have no corresponding browser-clock anchors. A source-consistent Home pointer is available for 99/100 primary switches and 8/9 slow switches: exactly two trusted pointer rows occur after the prior recorded action's second frame, and the second equals the exact destination-pointer reference. This ordered association lacks explicit target identity. The first primary switch has no prior action anchor and is not guessed.

Each of the eight associated slow actions has unique EventTiming pointerdown witnesses. Their Home click processing intervals are browser observations after dispatch, not CPU profiles or isolated React/storage durations. The five associated Home-dominant slow cases have click EventTiming durations 192, 144, 72, 136 and 136 ms, and processing intervals 110.2, 61.9, 19.6, 57.5 and 13.4 ms. These observations cannot explain the entire longer Node command or divide it into pre-pointer/post-pointer portions.

The three verification-tail cases (ordinals 7, 41 and 74) have associated successful specification-resource durations **28.1, 14.5 and 14.2 ms**, and orchestrator-resource durations **77.6, 39.2 and 36.2 ms**. Their destination-pointer→driver expected-DOM intervals are **273.6, 214.3 and 336.0 ms**. Responses end before the driver witness, but the witness follows a driver locator wait and evaluate; this does not establish rendering time, slow permission reads, or automation scheduling as the cause. Routes are recorded classes, not exact per-project request identities.

The separate eight follow-up switches have p95/maximum **251.833 ms**, with none exceeding 300 ms. Their separate trace is not used to explain the earlier slow nine. No overhead subtraction, clock alignment, outlier removal, adjusted threshold or additive percentile is used. Same-clock endpoint differences only describe explicitly named retained intervals.

The next code change should require deterministic proof in the measured Home path. Existing source inspection suggests checking redundant preference persistence and storage-wide navigation pruning, but this timing record does not attribute time to either. A fix to destination API/network speed is not supported by the three associated specification-resource durations alone. No specific product change is selected by this review.

`timings.all432.projection.json.gz` contains every original action timing and stage using an explicit allowlist, plus all 3,054 latency rows and 19,524 EventTiming rows. `switches.all108.json` keeps every primary/follow-up switch and its associations. `slow9-decisive-breakdown.json` retains the exact values summarized above. `source-binding.json` binds product receipt identity and independently hash-verified metric-helper source excerpts describing the boundaries. Quantiles use nearest rank and exactly reproduce the original per-cohort numeric summaries.

Original result SHA-256: `5f25447b1f6dc7eca50e220f358a4a029bd311e7d368e15ec85d693198569a00`. Original product commit: `972d6864a30b742b12545c8e12f4ca3ae2faa884`; the receipt reports a clean source. Raw input bytes were unchanged after analysis. Private paths, project identities, request URLs, bodies, fixture values and credentials are omitted from the published projection.

Reproduce offline with `python3 analyze.py --result <retained-private-result>`; the analyzer accepts only the bound input hash. It performs no app/browser/test/runtime operation. `ARTIFACTS.txt` declares the whole capsule; `manifest.json` and `SHA256SUMS` bind it.
