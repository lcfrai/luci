# CLOUD-032: 29931afc Long Animation Frame diagnostic

**Switching still fails:** the unchanged 100-switch primary cohort measured **494.921816 ms p95** against 300 ms. The 200-input cohort measured **86.268305 ms p95** against 100 ms in this run. All 400 primary actions and 32 separate trace-follow-up actions succeeded and remain retained. Wrapper exit 0 means capture completion, not full sustained or latency acceptance. No product change or measured performance improvement is claimed.

The exact installed 29931afc package ran from 15:40:32.617 to 15:43:32.331 UTC on 2026-10-09, including the original 60-second quiet phase. It retained the original action/verification/two-rAF timing, locators, saved-draft assertions and 300/100 ms p95 and <1000 ms p99 targets. No span was filtered or reduced. This is end-to-end automation timing, including RPC, scheduler and browser work, not physical input latency. No Linux sampler ran; all observer costs were not measured by a matched control.

## What the capture establishes

| Primary cohort | p50 ms | p95 ms | p99 ms | max ms |
| --- | ---: | ---: | ---: | ---: |
| 100 project switches | 267.905213 | 494.921816 | 825.795476 | 828.615283 |
| 200 inputs | 52.184953 | 86.268305 | 135.217758 | 195.641334 |
| 100 draft resets, separate context | 284.914669 | 555.513710 | 670.018074 | 670.455651 |

All 108 verification callbacks matched on their first sample; callback entry→resolve-request was at most 0.9 ms. That predicate does not explain the remaining switch tail. Among the 23 switches over 300 ms, the largest inclusive Node region was Open-project click in 15 and All-projects click in 8. Primary p95 regions were All click 141.44 ms, overview-heading wait 25.39 ms, Open click 176.48 ms, idea-field wait 57.26 ms, context evaluate 44.32 ms and final frames 43.84 ms. These separate quantiles are **not additive**.

All 200 trusted primary clicks joined exactly to EventTiming, without nearest-time matching. Home handler p95 was 48.0 ms; Open handler p95 was 16.5 ms. Same-browser-clock click→observed readiness p95 was 50.2 ms for Home and 81.1 ms for Define. Sixteen of the 23 slow switches had both reported click durations ≤100 ms. These browser observations do not remove the surrounding command, actionability, verification or frame costs from acceptance.

The bounded LoAF capture is valid: **66 frames, 50 script entries, 119 windows, 10 documents, no reported capture failure**. By phase wall-clock grouping, 44 frames began in primary actions, 3 in the separate trace follow-up and 19 during setup/outside declared phases. All 66 frames and all 50 script rows are published. Twelve of the 23 slow switches overlap a qualifying frame in an exact click, click→observed-ready, or Home-ready→Open-click interval; 11 do not. Every joined interval has a later snapshot, but a snapshot is not a continuous renderer watermark. No overlap does not prove idle rendering or transport causality.

The strongest actual renderer finding is original global row **124**, full switch 828.615283 ms. Its Home event handler took 212.9 ms. Frame `9:13` lasted 251.5 ms and contains a 211.1 ms app script with **91.8 ms forced style/layout**. Exact bundle SHA `526b6f92ddac31da933f04978f13f5bd5a3e2e5312422a04d89b8c53051c9c3e`, UTF-16 position 211123 in `web/assets/index-oRjgUib4.js`, resolves to React's discrete-event dispatch wrapper `_h`. It does **not** identify a component or layout-reading caller. Script/layout durations are reported wall intervals, not CPU samples. No speculative component patch follows from this evidence.

The separate follow-up trace contains 40 clicks in 8 extra rounds: 40 single attempts, 40 successful actionability waits, zero retry/instability/error records. Its Open actionability maximum is 37.09 ms; Home maximum is 39.79 ms. It does not trace the primary actions and supplies no evidence for a moving-card/CSS correction. Raw trace remains private.

All 1,420 allowlisted resource rows are retained in the 119 latency windows. They cover six route classes, without project IDs, URLs, request bodies or headers. Their durations are browser resource timings, not backend function timings, and do not establish a request caused a slow action. LoAF callback self-time totals 22.8 ms over 183 callbacks across 10 documents, counting each document's final cumulative snapshot once. This is partial callback accounting only; native observation, other observers, serialization and scheduling overhead remain unquantified.

## Closure and privacy

At **15:58:37.907 UTC**, independent read-only checks found the recorded driver, controller and main browser PID/start identities absent. Two recorded browser children remained literal same-identity **PID 1 zombies**; these are retained, not reported as absent. The explicitly selected Docker daemon had zero containers and zero volumes, matching engine/image identity; the declared fixture parent was empty. The 43-row journal was contiguous through driver-exit. No signals or mutations were issued. This covers recorded identities and the selected daemon/fixture parent, not a whole-machine census or unrecorded child absence.

`analysis/independent-closure.json` is distinct from the driver's zero-count cleanup receipt. `analysis/analysis-attempts.json` retains the first inline closure metadata-shape failure and a later read-only summary-printer error; neither mutated inputs or changed acceptance. Original inert helper test failure and correction remain in `accepted-preparation/`: 19 unique final successful controls, with repeated runs not added together. No tests/build/browser were rerun during post-run analysis.

The initial literal privacy scan flagged the unchanged public driver mode string; its failed receipt/source are preserved, and the corrected scan excludes only that known literal while explicitly checking all four original project IDs. No captured credential needles were found in the retained trace/result; this is not a universal secret-detector claim. The full result, actual Playwright trace, invocation/private fixture and raw request state are excluded. Their hashes bind the allowlisted projections. Published source helpers contain runtime credential-handling code, not captured credential values. The full source snapshot and old `runtimeLaunched:false` preparation fields are verbatim preparation-state evidence; successful runtime is established separately by the wrapper and run receipt.

## Exact bindings and reproduction

- Product/helper source: `29931afc5b31ae9b9bd7b8973a7e3fdb840609a3`.
- Private raw result SHA256: `d71194560d8c6852b2561062a236ef2af92d32feb917f624a507e28d0977e73f`.
- Private trace SHA256: `1a954cca6e7a4565dd3b6c4a447fd28753ec60915532fedb5b2a26bbc630e5ea`.
- Package/archive SHA256: `0b6eb1719c1ce4051eeb1cec55eef8625961f79cd2ffc1e14fb953a568fcefa0`.
- Binary SHA256: `d35e990d90f663772aa951bf19150e996680974cd5c627b8c8b7ce0da26314dd`.
- Driver SHA256: `82eacd50a9dea2c8c9ff1499a33f7d23f5799295d7fe0fa07874515304aae055`.
- LoAF observer SHA256: `db87926aa8fccb4db7df4391e789adb557d3920f78672baaac609d13ee9328c8`.
- Original frozen preparation manifest SHA256: `7a3b863bab3a3021d6892bec5e5ef4d1fa47ea0c15b6be022a3283228fbd9ddb`.

`accepted-preparation/launch-command.json` is the exact granted invocation, including package, image, helper/source pins, private fixture parent, selected daemon and output directory. Its referenced external preparation was copied byte-for-byte here, including the accepted driver/imports, tests, source bindings and launcher. To reproduce, restore those exact external paths/dependencies or make and record an explicit newly bound preparation; choose a fresh output and fixture. Never overwrite this attempt. Runtime requires its own coordinated exclusive window. `accepted-preparation/README.md` describes the original capture limits and controlled tests. None of this authorizes a rerun now.

Post-analysis used the prior published three analyzers with only input path/raw SHA rebinding, recorded in `analysis/analysis-execution.json`. The exact frozen join command is in `analysis/analysis-attempts.json`; the unchanged join helper is in `accepted-preparation/long-frame-joins.mjs`. `analysis/summarize-long-frames.py` validates the raw SHA and produces the allowlisted all-row derivation. These commands need the private retained raw input locally; the public projections preserve every numeric original action and captured frame/script for review without that private file.

Inspect `analysis/long-frame-analysis.json`, `long-frame-joins.json`, `event-timing-click-joins.json`, `switch-callback-joins.json`, `all432rows.csv` and `trace-click-projection.json` for all-row decisions. `analysis/NEXT-ATTRIBUTION.md` prepares one finite eight-round Layout-stack/invalidation diagnostic and a separately gated hidden-map equality check. It is a plan, not an executed trace or component finding. Hidden maps alone cannot recover a missing nested caller stack.

Validate publication bytes with `sha256sum -c SHA256SUMS` from this directory. The nested frozen preparation manifest remains unchanged and independently verifiable from its directory. `.gitattributes` preserves all capsule bytes verbatim.
