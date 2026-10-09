# Narrow primary-cohort diagnostic — prepared, not run

The original primary-cdp-1 attempt remains FAILED and immutable. All 400 action results and a 1,097,185-byte CPU profile were retained, but `Tracing.tracingComplete` reported data loss and the export exceeded 32 MiB. Only 140,324 complete events could be decoded from that retained prefix. It contains frame markers for primary ordinals 1–151, not the complete400. These observations do not certify full tracing or fixture cleanup.

Prefix byte weights explain the capture change: `devtools.timeline` accounts for about22.26 MB, `v8`4.03 MB, timeline+GC3.48 MB and dedicated GC1.14 MB, versus about365 KB for `blink.user_timing`. Prefix categories cannot establish whole-run proportions or the trace's missing events. The narrower candidate records only `blink.user_timing`; the existing whole-window CPU profile remains. Buffer16MiB, output32MiB, CPU12MiB, capture150s and the original workflow deadlines are unchanged. No bounds or workload thresholds were increased.

The candidate changes only `primary-profiler.mjs` and `latency-diagnostics.mjs` from the first prepared candidate. All400 action ordering/timing, the driver, interactions, task-command and seven frozen runtime dependencies remain byte-identical. The observer adds optional long-task and long-animation-frame numeric timing entries during the primary window. Unsupported entry types remain explicit. At most64 script timing entries per long frame contribute to a numeric summary; omitted entries are counted. No source URL, function name, attribution container, DOM content or credential is collected by those observers. They cover only tasks/frames over50ms, so absence cannot establish zero renderer work. No extra per-action RPC is added.

`analyze.py` joins exact primary ordinal markers and same-document passive timing. It reports Node stage durations without subtracting browser-clock values, shows browser marker offset drift and exposes overlap of known resource classes and long tasks/frames with the post-dispatch portion of each slow cohort. The switch witness starts at the final project-open pointer, not the preceding overview click. Style/layout/paint categories excluded from tracing are marked **not captured**, never reported as measured zero. CPU is whole-window statistical evidence; `(program)` means unknown/native, not app CPU. Minified bundle locations do not establish a named React component cause. Remaining actionability/transport gaps require a separately labelled Playwright tracing arm, with its throttling and privacy effects disclosed.

Root may launch only after the sustained run has settled and the staged pure checks have run:

```bash
source /workspace/.luci-tools/activate.sh
cd /workspace/.luci-tools/cloud032-primary-profiling-narrowing
node --test primary-profiler.test.mjs passive-observer.test.mjs
python3 test_short_socket.py
python3 test_analyzer.py
node --check candidate/sustained.mjs
node --check candidate/latency-diagnostics.mjs
node --check candidate/primary-profiler.mjs
python3 launch.py --helper-source EXACT_CLEAN_40_HEX_COMMIT --run-name primary-markers-1 --execute
```

No runtime or checks above have yet run for this narrowed candidate. The launcher retains exact product2da package/image arguments, new clean helper checkout authorization, all frozen helper/tooling hashes, and fresh short0700 fixture-parent allocation with both Unix socket path checks. It does not reuse the failed fixture. Raw traces/profile/results stay private. Run the original independently bounded cleanup/identity review before claiming fixture settlement; diagnostic detach alone is insufficient.

After settlement:

```bash
python3 analyze.py \
  --receipt runs/primary-markers-1/result/report.json \
  --profile-dir runs/primary-markers-1/result/private-primary-profile \
  --output runs/primary-markers-1/allowlisted-analysis.json
```

`failed-prefix-analysis.json` was computed by an earlier inline Python command, before sustained5. `reproduce-failed-prefix.py` reconstructs that computation against the exact hash-bound inputs and writes a new output only; it is staged, not yet executed. Do not claim its source hash belongs to the original inline execution. `(program)` is unknown/native throughout both analyses. No acceptance pass, overhead subtraction, performance improvement or causal conclusion follows from either instrumented run.
