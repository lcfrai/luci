# Public API-name span diagnostic — prepared, no runtime launched

This is a separate instrumented diagnostic after the complete marker-only run. Preserve both prior runs and their original timings. Product/package stays2da; root supplies the exact clean current helper commit. All frozen runtime dependencies, source guards, fresh short fixture parent and cleanup fences remain.

The pinned Playwright public `launch({logger})` accepts a Logger inherited by contexts/pages. Its client source emits `=> apiName started` and `<= apiName succeeded/failed` with empty args. Public typings explicitly mark Logger deprecated because logs are incomplete. This candidate therefore requires an exact observed call sequence for all400 primary actions and refuses absent/ambiguous coverage. It does not claim all internal polling is logged.

The logger starts disabled, enables during the400-action phase, and records only while one original measured action has an explicit ordinal. It stores ordinal, allowlisted API name, started/succeeded/failed and offset in the same Node `performance.now()` clock as that action. It never inspects args, hints, Error contents, selectors, results, values or arbitrary message content. Message matching is strict and bounded. Setup/credentials and between-action calls are not collected. Child DEBUG/DEBUG_FILE/PWDEBUG/PWDEBUGIMPL are removed to avoid an inherited private debug sink; user environment is not changed.

No Playwright tracing, throttling override, new action RPC or product state mutation is added. Original action start/end clocks, assertions, ordering and300/100ms thresholds remain. Observer bookkeeping adds overhead and must never be subtracted or used to claim an acceptance pass.

Bounds:400 actions,8000 entries,2MiB compact entry bytes,100 diagnostic problem codes. Overflow, changed ordinals, unmatched/overlapping calls or incomplete exact sequences fail diagnostic completeness. Existing CDP bounds and independently bounded resource cleanup remain. Partial logger evidence is preserved by the original finally path. Expected full coverage is3600 callbacks/1800 public calls:700 calls for100 switches,600 calls for200 input actions,500 calls for100 draft resets.

Ten final pure logger tests passed, including exact400 sequences; ignored outside-window activity; poisoned args/hints that throw if read; message/secret/newline rejection; missing callbacks; byte/entry caps; overlap/pending calls; failed calls; changed ordinal; invalid/backward clock. The initial9-case run is retained and overlaps the final10, not19 distinct tests. Syntax checks passed. These are helper tests, not actual browser callback coverage.

After root selects an exclusive window:

```bash
source /workspace/.luci-tools/activate.sh
python3 /workspace/.luci-tools/cloud032-public-api-logger-preparation/launch.py \
  --helper-source EXACT_CLEAN_40_HEX_COMMIT --run-name public-api-spans-1 --execute
```

After the run and independent cleanup:

```bash
cd /workspace/.luci-tools/cloud032-public-api-logger-preparation
python3 analyze-api-spans.py --receipt runs/public-api-spans-1/result/report.json \
  --output runs/public-api-spans-1/public-api-analysis.json
python3 analyze.py --receipt runs/public-api-spans-1/result/report.json \
  --profile-dir runs/public-api-spans-1/result/private-primary-profile \
  --output runs/public-api-spans-1/allowlisted-analysis-v3.json
```

The API analyzer is prepared but not actual-run verified. It validates exact ordinal/order/state pairing and keeps every callback span within its original Node action duration. It exposes the idea-visible wait separately from value evaluation/read and frame observation, without assigning the gap to renderer or backend. No browser/Node subtraction. Raw profile stays private; only allowlisted summaries/hashes may be reviewed for publication.

The copied v3 marker analyzer preserves and discloses negative raw CPU deltas, withholding weighted bundle/CPU attribution without clamping, reordering or taking absolute values. The earlier preliminary weighted CPU projections remain historical and must not be used for causal attribution. Negative deltas alone do not establish a browser, SDK or clock bug.
