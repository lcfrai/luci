# CLOUD-032: public API logger cannot identify the slow-click wait reason

The retained `public-api-spans-1` capture provides **no attributable repeated stability, visibility, interception, locator-resolution or dispatch wait condition** for slow All projects/Open commands. Its accepted logger stored only `{ordinal, api, state, nodeOffsetMs}` for public call start/success/failure. It did not retain Playwright's internal actionability messages, selectors, arguments or hints. Even a complete public call pair would not distinguish these internal phases.

Coverage failed before any of this run's switches over 300 ms:

- 400 original actions and 137 retained API callback entries.
- Actions 1–15 have complete public lifecycle sequences: four switches, eight inputs and three resets.
- The four covered switches are ordinals **1, 5, 9, 13**, with full action durations **271.417197, 235.130432, 238.502292 and 217.724457 ms**.
- Action 16, a draft reset, has only three entries; actions 17–400 have none.
- All **12 switches over 300 ms have zero logger entries**. Their ordinals are **21, 37, 77, 105, 133, 177, 241, 301, 329, 345, 373, 389**. All 100 switch rows, original full durations and Node stage marks remain in the derivative.

The original result remains failed: `complete:false`, `overflow:true`, exactly 100 retained `unrecognized-api-message` problem codes, and 12,293 retained entry bytes against a 2 MiB byte bound. The configured 100-problem limit was reached; the byte limit was not the failure. Unknown callback text was never retained, so its exact historical classification cannot be recovered. A later controlled background-callback explanation is not evidence of which messages caused this original failure. No unknown code is reclassified or treated as successful logging.

No Playwright actionability trace was enabled in this attempt. Its separate CDP marker/CPU capture does not supply the missing actionability records; the CPU timing was independently rejected because it contained negative deltas. Other runs' follow-up actionability traces describe different episodes and cannot fill this gap. The diagnosis stops here: the retained evidence does not resolve the slow commands' internal wait reason. No new harness, runtime, test, profiler, product edit, overhead subtraction or acceptance reinterpretation follows.

Product/package source was `2da1a821b8456c20a858fb86ef87649ea917d9de`; the helper checkout guard was `765fcaadeee7de1cf2ba3a3fb7b295d0af445b70`. Instrumented switch p95 was 363.614553 ms, still above the unchanged 300 ms reference. This failed diagnostic does not replace acceptance.

The retained private original report was read and SHA-256 verified as `c7b6f65b4e219a1f084e98774158ff57a2f1327e088e1b7371860fe076c358a5` (7,126,776 bytes). Its `publicAPISpans` subtree exactly equals the published projection. The original was not copied or changed. The accepted logger source has SHA-256 `994468a342b44b4bb41294242ccfcad1281331d5f59229e1cd0f05bc9e5a09f9`, matching the preparation source.

`derived/original-logger-projection.json` preserves the exact existing safe projection, including all 400 action coverage records, all 137 callbacks and all 100 failure codes. `derived/accepted-public-api-spans.mjs` preserves the exact executed logger. `derived/assessment.json` retains all 100 switch joins and explicit unavailable wait reasons, with exact input hashes. Only the original safe enums/numbers and fixed diagnostic text are emitted; no project names, context values, private fixture locations, URLs, headers or unknown messages are added.

Reproduce using a fresh external output directory:

```sh
python3 analyze.py \
  --input /workspace/luci/docs/luci/evidence/cloud032-primary-diagnostics-20261009/public-api-spans-1 \
  --output /tmp/cloud032-public-api-waits-FRESH
```

The bounded script verifies three exact public input hashes and the decoded logger hash. It does not need the private original, launch any process under test, repair coverage or run the original analyzer that correctly refuses this incomplete capture. Raw analysis stdout/stderr are retained; the offline derivation completed with exit 0 and empty stderr. That exit status concerns this coverage inspection, not the failed original logger.
