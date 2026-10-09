# Prepared primary-cohort diagnostic — not executed

Root selects an exclusive window after C041 work settles. This first arm uses CDP CPU sampling and timeline recording only. Playwright tracing is intentionally disabled because the installed implementation changes throttling during trace capture and retains API parameters/results. That separate arm is justified only if click/actionability waits remain unexplained.

Run from the clean, newly committed helper checkout without changing its product source or helper files:

```bash
source /workspace/.luci-tools/activate.sh
python3 /workspace/.luci-tools/cloud032-primary-profiling-preparation/launch.py \
  --helper-source EXACT_NEW_CLEAN_40_HEX_COMMIT \
  --run-name primary-cdp-1 --execute
```

`launch.py` has not been executed. It requires an explicit immutable helper commit, clean checkout, exact frozen candidate/tooling hashes, and the original exclusive-empty-Docker fixture contract. It preserves product/package/image arguments from the prior 2da invocation and writes a fresh private invocation containing every exact argument. The original driver independently checks package contents, image identity and clean separately declared helper revision before app startup. The helper commit may differ from product `2da1a821b8456c20a858fb86ef87649ea917d9de`; the receipt states this explicitly.

At launch only, a fresh 0700 `mkdtemp` parent `/workspace/.lp-XXXXXXXX` holds the app fixture. No old `.s` directory or fixture is traversed or reused. The private run directory retains its exact parent path/device/inode/UID/mode and creation time in `fixture-parent-identity.json` and the invocation. Both frozen Unix socket templates (`docker-proxy.sock` and `.luci-fence-proxy-XXXXXX/docker.sock`, under the driver's `luci-sustained-XXXXXX`) are checked in UTF-8 bytes against the exclusive 108-byte limit before app startup. Six pure tests verify the original too-long parent is rejected, the exact 107/108-byte boundary, multibyte input, invalid paths and frozen Node call sites. These tests allocate no fixture or socket. The old unexecuted launcher is preserved as `launch-before-short-socket.py`; the new short parent remains available for explicit final owned-fixture closure review, rather than automatic reuse or broad deletion.

All seven runtime helper dependencies are exact captured bytes from the accepted 2da idle run. They are copied beside the driver, including the transitive startup → lifecycle import. The frozen task-command has only a `node:assert` import; no C041 source is imported. Driver capture and final verification read these frozen copies and label their original paths. Clean-checkout guards remain active. Installed Playwright/playwright-core JS, MJS, CJS and JSON files are independently hashed as tooling, not attributed to the product revision. A changed tooling hash refuses launch.

The original workload stays 60 seconds quiet followed by 100 project switches, 200 input actions and 100 draft resets, in the same order. Original `performance.now()` action starts/ends, stage boundaries, DOM assertions, two animation frames, queue pause and zero Create/Start/provider assertions remain. The optional 32 post-primary trace actions are disabled (`--trace-after no`) for this first diagnostic arm. No extra per-action RPC is added: the existing frame RPC receives its primary ordinal and emits three marker witnesses. Existing passive callbacks emit fixed-name/sequence timing markers. These additions have overhead; instrumented numbers cannot replace the original 300/100 ms p95 acceptance thresholds, be corrected by subtracting overhead, or establish causality.

Bounds: 150 seconds of recording within the unchanged 210-second workflow, 16 MiB `recordUntilFull` trace buffer, 32 MiB retained timeline, 12 MiB serialized CPU profile, 5 ms CPU sample interval, 256 trace-read chunks, 9-second internal finish and 10-second outer finish. Capture timeout, full buffer, data-loss report, unresolved protocol command, stream cap or failed detach produces an incomplete diagnostic. The original independently bounded controller/browser/provider/proxy cleanup follows even when profiling fails. The 450-second wrapper records timeout without claiming cleanup or signaling unrelated processes. CDP can return a CPU object before its serialized size is checked; the output cap is not a promise of synchronous protocol-memory preemption.

The three modified files are `sustained.mjs`, `interaction-metrics.mjs`, and `latency-diagnostics.mjs`; `primary-profiler.mjs` is additive. `diagnostic.patch` and `preparation.json` describe all frozen-source differences. The original accepted files and prior evidence remain unchanged. No product source changes, provider calls, browser launches, Docker calls or application fixtures occurred during preparation. Eleven pure fake-transport tests exercise bounded startup, late handle closure, normal/failed completion, unresolved commands, data loss/full buffer, chunk/byte bounds and capture expiry. They are helper logic evidence only.

After the run and resource settlement, a bounded secondary analysis is available:

```bash
python3 /workspace/.luci-tools/cloud032-primary-profiling-preparation/analyze.py \
  --receipt /workspace/.luci-tools/cloud032-primary-profiling-preparation/runs/primary-cdp-1/result/report.json \
  --profile-dir /workspace/.luci-tools/cloud032-primary-profiling-preparation/runs/primary-cdp-1/result/private-primary-profile \
  --output /workspace/.luci-tools/cloud032-primary-profiling-preparation/runs/primary-cdp-1/allowlisted-analysis.json
```

The analyzer has not run against an actual capture. It requires the complete ordered primary400 and exact unique marker joins. It reports slow cohorts, existing Node stages, browser dispatch→DOM/frame intervals, timeline categories and whole-window CPU bundle locations. Marker offset drift is exposed. Browser and Node clocks are never subtracted. Category unions can overlap and must not be summed; absent event names are not evidence of zero work. The project-switch post-dispatch witness begins at the last project-open pointer, not the preceding overview click. Whole-window CPU samples are not assigned to action windows without further verified clock alignment. Minified locations without production sourcemaps do not prove a named React component is responsible.

Raw timeline/CPU/receipt files stay private (directory 0700, data 0600). Disabling network/screenshots/DOM categories does not guarantee arbitrary trace args or source URLs contain no identifying data. Publish only reviewed allowlisted summaries and hashes, never the raw trace, private fixture credentials or full API parameters/results. The analyzer intentionally omits URLs, function names, source text, request contents and project IDs; root still reviews any proposed publication.

Root-supplied inputs: exact clean helper commit after C041 (source truth), fresh run name (no overwrite), exclusive runtime window (avoid cross-workload interference), and independent final owned-process/Docker/ledger closure (profiling transport detach alone is not fixture settlement).
