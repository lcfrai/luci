# CLOUD-032: installed headers-only diagnostic, product 990 / helper a31

The controlled diagnostic completed all 320 actions. **Both switch conditions still fail the unchanged 300 ms p95 reference.** This run does not establish a latency improvement, isolate a frontend cause, justify a product optimization, or provide end-to-end/performance acceptance.

Product source is `99007404a5ddd8592dee376ea00d99c6180d26fe`; the separately declared clean helper checkout is `a31e1ac485c95bdbcc9ebb011581da5ce3f5c67a`. The current 67a5 product is not the measured package. The original result SHA256 is `876c638211523c48f1657cc723169c0ce35b9dd24d065269b95a179488ad5bdd`. Its raw bytes remain private and unchanged outside this capsule ; [original-artifacts-metadata.json](original-artifacts-metadata.json) records sizes and digests. [result-projection.json](result-projection.json) is an explicit allowlist projection, with opaque project identifiers mapped to A/B/C/D. It retains every original measured span and its stage/frame diagnostics, every eligible request/response join, passive browser/resource observations, long tasks, identity checks and bounded cleanup assertions.

## Original spans and unchanged references

Each arm has 40 project switches, 80 input actions and 40 draft resets; all succeeded. Quantiles use the original nearest-rank formula `ceil(n*q)-1`, recomputed from all 320 rows and checked against the original summary. Spans include automation transport, expected saved-context verification and two animation-frame callbacks; they are not physical display latency.

| Action | Condition | n | p50 ms | p95 ms | p99 ms | max ms |
| --- | --- | --- | --- | --- | --- | --- |
| project-switch | baseline | 40 | 250.245 | 599.779 | 613.584 | 613.584 |
| project-switch | defer | 40 | 246.571 | 577.614 | 780.484 | 780.484 |
| input | baseline | 80 | 49.653 | 72.599 | 295.594 | 295.594 |
| input | defer | 80 | 49.874 | 69.693 | 192.912 | 192.912 |
| draft-reset | baseline | 40 | 269.477 | 349.829 | 834.099 | 834.099 |
| draft-reset | defer | 40 | 266.949 | 697.063 | 713.345 | 713.345 |

Both input p95 values are below the 100 ms reference and all p99 values below 1000 ms under this diagnostic intervention. There is no separate original draft-reset p95 target. The result's `passed:true` means the bounded conditional diagnostic completed ; `diagnosticOnly:true` and `fullSustainedAcceptance:false` remain explicit.

Switch p95 differs by −22.165 ms, while switch mean increases from 290.940 to 300.336 ms and p99 rises from 613.584 to 780.484 ms. Draft-reset p95 rises from 349.829 to 697.063 ms. Of40 matched project/block switch pairs, defer is faster 22 times and slower 18; the paired mean difference is +9.396 ms. Five block means improve and five worsen. This mixed single-run description is not a population estimate or significance result.

## Project and block balance

All four projects contribute ten switches per condition. The frozen seeded schedule contains ten eight-round blocks; each block has one baseline and one defer round for each project. Per-project p95 and p99 with n=10 equal the maximum; per-block p95 and p99 with n=4 also equal the maximum. These small-tail summaries should not be interpreted as stable estimates.

| Project | Baseline switch p50/p95 ms | Defer switch p50/p95 ms |
| --- | --- | --- |
| A | 249.803 / 498.525 | 241.977 / 450.488 |
| B | 248.216 / 358.413 | 225.701 / 589.514 |
| C | 251.368 / 613.584 | 221.218 / 534.405 |
| D | 249.419 / 607.863 | 248.844 / 780.484 |

| Block (zero-based) | Baseline switch mean ms | Defer switch mean ms | Defer−baseline ms |
| --- | --- | --- | --- |
| 0 | 302.398 | 284.698 | -17.699 |
| 1 | 261.344 | 359.531 | 98.187 |
| 2 | 264.124 | 254.308 | -9.816 |
| 3 | 249.058 | 313.132 | 64.074 |
| 4 | 261.556 | 307.155 | 45.599 |
| 5 | 360.136 | 228.522 | -131.613 |
| 6 | 335.931 | 285.938 | -49.994 |
| 7 | 214.360 | 379.235 | 164.875 |
| 8 | 326.693 | 215.458 | -111.235 |
| 9 | 333.803 | 375.387 | 41.584 |

[analysis/summary.json](analysis/summary.json) retains p50/p95/p99/max/mean for every action kind by condition, project and block, all 40 matched switch pairs and all 80 round exposure records. [analysis/actions.csv](analysis/actions.csv) provides the320 original spans with round, block, project and condition labels.

## Actual exposure and cancellation

Every measured round has exactly five eligible 200 responses: four project overviews and one host-capacity/scheduler response. Baseline continues all 200 original responses, and all 200 network requests finish. Defer holds all 200 responses; **every held request is naturally aborted by the browser**, then resolves only through the matching cancelled network identity and an invalid original interception ID. No held response later delivers its body. There are no unexposed rounds, missing request/response joins or cross-round assignments.

This is response availability/backpressure **and cancellation**, not deferred frontend commits or delayed delivery of identical completed bodies. No body capture, substitution, replay or retry occurred. Server-side body completion, backend work and cancellation cost are not established. The same original Playwright permission/origin/mutation guard remains byte-identical in both arms.

Baseline pause-record lifetime p95 is 5.822 ms (max 8.517); defer p95 is 1042.702 ms (max 1576.422). For cancelled requests, this record lasts until the end-round terminal-ID resolution; it is **not** the time the already-cancelled network request remained blocked. Original Network request-to-terminal p95 is 67.069 ms baseline and 274.689 ms defer, calculated only within the CDP Network clock. Browser resource timing and Node timestamps are never subtracted from each other.

## Long tasks and stage observations

All 28 observed long tasks are retained with a continuous sequence and no drop/unsupported flag. Twenty overlap at least one controlled action's same-document browser interval. Six precede the first measured dispatch (setup/quiet); two fall outside the observed action intervals. A snapshot taken after a round can contain earlier tasks, so snapshot labels are not used to assign tasks to conditions.

An observed switch interval starts at the first of the exact two scripted trusted pointer dispatches and ends at the second animation-frame callback. Input intervals start at their trusted input dispatch; draft reset starts at the first of its three scripted pointers. The source-derived pointer counts and chronological sequence were checked. These intervals exclude automation waits before dispatch; they are not full Node spans, React commits, physical paint, or CPU ownership attribution.

| Action | Baseline intervals with overlap | Defer intervals with overlap |
| --- | --- | --- |
| project-switch | 4 / 40 | 8 / 40 |
| input | 1 / 80 | 0 / 80 |
| draft-reset | 1 / 40 | 4 / 40 |

The unchanged Node stage observations remain descriptive: slow tails occur in multiple stages, and the intervention does not remove switch delays. For example, All-projects click p95 is 137.095 ms baseline versus 301.234 ms defer; saved-context verification is 110.146 ms versus 87.623 ms. Stage quantiles do not add to a whole-span quantile. All passive resource observations, including setup/quiet/drain, remain available by route class; they are not function timings or exact request-body completion evidence.

## Identity, guard and cleanup scope

The original archive SHA is `9bf32d8ebe0c7c7d3297c81e91f0451f0035357cb77eb7abf06099920dd9f164`; the driver verified 954 members, all matching. Exact accepted helper bytes match the frozen v2 preparation. Original source/context/permission guards, paused queues, zero-Create/Start fences and original timing targets remain in force. All ten browser mutations are setup/cancellation-protection actions outside the measured phases; recorded Create, Start, worker, model, provider and auxiliary counts are zero. Explicit local setup actions are not represented as a read-only whole run.

The driver records closed interception, no paused IDs, no cleanup errors, completed controller/browser/provider/proxy cleanup and removed fixture. [independent-closure.json](independent-closure.json) independently checks exact recorded driver/controller identities absent and empty exclusive Docker/volume inventories, with original artifacts unchanged. It does **not** provide an independent browser PID/start census or whole-machine closure. Driver-owned browser-zero and fixture-removal statements are retained as driver assertions, not promoted to independent facts.

## Preserved preparation and failed alternatives

[preparation-v2/](preparation-v2/) preserves the exact sealed external generator, launcher, driver, original/imported helpers, fixed schedule, plan, source/package bindings, eighteen-check headers-only transport proof and all raw inert validation. Its manifest SHA is `fae7a84d5ec5ef9bbdf66f71594a63f5c0374cf512a9d2ef7a091da642ac17e8`. Initial eight manager cases passed; the added ninth receipt-join case failed before the correction, then the same nine passed. The failure and source variants remain unchanged. Two pinned-input refusal checks and final syntax/AST results are included.

[prior-v1/](prior-v1/) preserves the old full-body plan, original eighteen-check proof, failed pre-body cancellation proof, blocker/status and exact draft sources. The pre-body body-capture command failed to settle until owned browser closure; this is not relabelled as passing. The v2 eighteen checks are separate, not a cumulative count. Prior prepared plans say “not executed” according to their seal time; this publication adds the later installed result without altering those historical records.

Only source, synthetic controlled proof data, validation evidence and allowlisted installed observations are copied. Original private installed result/stdout/journal/image-provenance and fixture details stay outside the capsule. Package locations and source-code path placeholders in frozen evidence are preserved; they do not expose actual private fixture directories. Publication does not assert every historical generator revision was captured.

[analysis/analyze.py](analysis/analyze.py) reproduces the projection and analysis from the original result, refusing a mismatched SHA. [analysis/command.json](analysis/command.json) records this offline analysis invocation. No new installed run, benchmark, browser, provider, build or test was started for this capsule.
