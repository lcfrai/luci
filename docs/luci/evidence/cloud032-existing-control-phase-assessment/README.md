# CLOUD-032: existing static control and full-action command regions

The retained matched **two-click static control does not exceed 300 ms**. Its 100 primary switches have p50 **111.319830 ms**, p95 **157.082192 ms**, p99 **225.692975 ms**, maximum **288.855712 ms**; all succeeded and none exceeded 300 ms. Its eight separate traced switches have p95/max **110.625189 ms**. All 432 original static scalar action rows are retained in `assessment.json`, grouped without dropping inputs, resets or follow-up actions.

This is the original `cloud032-switch-latency/static/result.json.gz` receipt, decoded SHA-256 `ed5cd4d8c2dc75a13f085642537e43aa8d326909ca56bac6c798a56fb9e8127b`. The d348 reference repeats that same receipt. The bounded review of the other named C032 idle comparison capsules found actual application arms, not another two-click static run above 300 ms. The separate settled-editor control consists of 32 verification evaluations and cannot answer the full two-click question.

The static fixture runs the original two ordinary clicks, heading/value checks and two-rAF completion through the exact original interaction helper. Its HTML has synchronous DOM replacement and no React, persistence or product APIs. The primary 100/200/100 counts, 60-second quiet interval and original action deadline were retained. The three directly relevant static sources are pinned by this assessment; independent source review also checked all seven accepted source files against the original receipt. This was a historical sequential static/actual comparison, **not** a current b94/299 matched-load run or universal automation floor. Its cost cannot be subtracted from application timings. It does not prove an intrinsic 300 ms failure in the measurement mechanism, nor prove that current host scheduling is adequate.

## Where complete application actions spend their recorded time

The existing primary Node command regions concentrate the descriptive maximum in clicks:

| Exact historical primary cohort | Switch p95 | Switches over 300 ms | Largest region among those slow switches |
| --- | --- | --- | --- |
| 29931afc, 100 switches | 494.921816 ms | 23 | Open click 15; All projects click 8 |
| 5f4a966f, 100 switches | 579.139597 ms | 37 | Open click 25; All projects click 9; heading wait 1; field wait 1; input-value assertion 1 |

Across all 100 primary switches, the largest region is Open/All projects in 66/34 rows for 299 and 72/25 rows for 5f4, with the same three other single-row categories in 5f4. `assessment.json` retains all 216 original primary/follow-up switch region rows and recomputes these classifications. Follow-up phases remain separate.

| Inclusive Node region | 299 primary p95 | 5f4 primary p95 |
| --- | --- | --- |
| All projects click | 141.441563 ms | 126.407641 ms |
| Open project click | 176.476989 ms | 188.135784 ms |
| Overview-heading wait | 25.390970 ms | 46.726960 ms |
| Idea-field wait | 57.255231 ms | 62.478682 ms |
| Context evaluation | 44.321392 ms | 51.692411 ms |
| Input-value assertion | 13.205838 ms | 15.186330 ms |
| Final two-frame RPC | 43.844605 ms | 40.432464 ms |

These quantiles are **not additive**. Each region includes the original command's lookup, actionability, protocol/driver/browser scheduling, application work and settlement where applicable. The label is not a causal attribution. There are genuine isolated non-click tails too; for example, 5f4's field wait reaches 367.135288 ms and input-value assertion reaches 241.481322 ms. They remain in the complete distributions.

All 108 original callback receipts in both 299 and 5f4 matched their predicates on the first sample. The separate frozen b94 EventTiming derivative retains 16 unique click joins with reported click durations 48–80 ms and a maximum reported interaction-group duration of 88 ms. These facts narrow particular hypotheses but do not isolate the unaccounted portions of full commands. The old 299 forced-layout outlier is real and still unattributed to an application caller; it was not reproduced by the eight-round b94 capture. None of this establishes an application render bottleneck, transport floor or CSS defect.

## Concrete next decision

Proceed with the already justified C076/C078 product corrections and their required correctness validation, then measure the **matching frozen package with the existing unchanged idle harness**, in the root-coordinated exclusive window. Retain all original 100 switches, 200 inputs, 100 resets and 32 separately labelled trace actions, the 60-second quiet interval, original guards, action sequence, full spans and 300/100 ms targets. Bind the exact package, helper and dependency bytes. This is an acceptance check of the new actual package; it needs no new observer, generic profiler or static comparison.

C078 avoids independently proven idle notification/persistence work. C076 suppresses attach attempts only while a recorded allocation is not ready; an idle fixture with no eligible preparing attachment may not exercise that change at all. Neither establishes a latency improvement. Report whatever the new package measures without attributing a before/after difference to either correction or adjusting any duration. If it still misses, retain that result and keep CLOUD-032 open; do not spend another iteration on speculative memoization, CSS, callback polling or another generic trace. A subsequent product correction needs an identified source operation, not merely a command-region label.

## Reproduce the bounded assessment

```sh
python3 assess.py \
  --evidence-root /workspace/luci/docs/luci/evidence \
  --output /tmp/cloud032-control-phase-FRESH.json
```

This reads eight named hash-pinned public files, enforces compressed/decoded size bounds, recomputes nearest-rank quantiles and largest-region counts, and refuses an existing output. It does not launch tests, a browser, the application or a profiler. The exact input hashes, all 432 static scalar rows, all 216 historical switch region rows and separate cohort summaries are in `assessment.json`. Original captures, earlier frozen analysis, repository sources and acceptance criteria remain unchanged. The current assessment has no runtime result, product gain or performance-pass claim.
