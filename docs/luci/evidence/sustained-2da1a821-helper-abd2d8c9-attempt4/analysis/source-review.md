# Sustained attempt 4: incomplete workload and unchanged latency failures

Exact packaged product: 2da1a821b8456c20a858fb86ef87649ea917d9de. Exact helper checkout: abd2d8c9314d6b73d0c202c82de72f3c3acc0d9a. This is a failed installed Linux scripted-provider experiment, not a completed sustained or real-model acceptance run.

The original driver catches `Actual tool output must supply completion receipt or live command identity` in its provider handler at sustained.mjs:302, while the retained stage is `active-command-restart` (2026-10-09T02:07:37.419Z). That stage name does not establish that the controller restart failed. Its earlier recorded witness shows the same container, PID, reviewed context and one command dispatch through the 02:04:31.326–02:04:34.223 restart. Both terminal probes record zero exit frames. The final receipt has zero accepted task results and no natural completion, integration, successor admission, amendment, settled calibration or full functional result.

The command event log records expected-red failure, successful build, successful fixed tests and artifact behavior, and a fuzz command exiting zero after 188.374 seconds of wall time. Separate original-output triage (CLOUD-041) retains the complete native command receipt and committed task result: 150 active fuzz seconds and 6,846,700 executions, followed by an actual clean task commit. The model-facing tool output was truncated and omitted the receipt marker/live identity required by this fixture handler. Those retained command results do not upgrade the missing accepted task response or establish a natural native session completion. No duplicate dispatch, new task success or recovery is inferred from that output.

## Interaction evidence

All 696 recorded actions succeeded: 400 idle actions and 296 partial-loaded actions. Exact numeric Node stage and browser-frame diagnostics are retained for every row. Independent nearest-rank calculations match all six original phase/kind groups.

| Phase | Switches | Inputs | Switch p95 / p99 ms | Input p95 / p99 ms |
|---|---:|---:|---:|---:|
| Idle | 100 | 200 | 385.298971 / 460.619650 | 64.824527 / 132.009124 |
| Partial loaded | 74 | 148 | 550.770057 / 1445.939623 | 68.005099 / 321.629731 |

The unchanged 300 ms switch-p95 target fails in both groups. The loaded switch-p99 also fails the 1000 ms ceiling. Input p95 meets 100 ms in these recorded groups, but the loaded minimum of 200 switches/400 inputs is not reached, and there is no final settled phase. Draft-reset rows remain intact and separately labeled. No rows, probe cost or restart period are removed to change an outcome.

Eleven idle and 39 of 74 loaded switches exceed 300 ms. Loaded median/p95 All-projects click-call durations are 107.602/302.594 ms, compared with 67.184/130.190 ms idle. Independent component quantiles must not be added. The worst loaded switch (1445.940 ms, 02:04:53.814Z) spends 1139.852 ms in the Open-project click call; its temporal browser click handler is only 18.4 ms, while EventTiming duration is 944 ms and Open-to-expected-DOM is 1145.7 ms. Another loaded switch spends 1053.241 ms in the All-projects click call with only 42.6 ms matched click handling. These are evidence of delay outside the short matched handler spans, but cannot distinguish rendering, scheduling, automation transport/actionability or system effects. No primary CPU/GC/style trace exists, so there is no defensible React-function causal attribution or justified additional blind component change from these rows alone.

The metric includes automation commands, DOM verification and two animation frames. It is not physical input-to-display latency. This attempt includes the Linux observer and a real worker workload; earlier idle-only runs lack that sampler and are not a matched causal control. The full original workload and thresholds remain required after the fixture correction.

## Resource observations

There are 414 approximately-one-second samples, with 387 valid control-plane cgroup observations and 27 not-observed samples. Recalculation of every observed physical/visible-ancestor minimum yields zero formula mismatches. Minimum effective available memory is 10,243,493,888 bytes; minimum physical available memory is 21,255,237,632 bytes. These sequential minima are not atomic admission values or Luci-only usage. The finite host cgroup also includes unrelated tasks, retained RAM-backed files and observation work.

The one container appears in 223 identity rows and 220 usage rows, with sampled peak cgroup usage 446,545,920 bytes. Owned host-process RSS peaks at 1,362,939,904 bytes (shared pages can be double-counted); sampled browser RSS peak is 1,033,093,120 bytes and control-plane peak 173,973,504 bytes. Container usage is separate and must not be indiscriminately added to overlapping process measurements. These partial-run peaks are not a four-project memory-growth or whole-computer responsiveness pass.

Observer interpreter self-cost is approximately 0.193934 core over 418.039 seconds, excluding spawned Docker CLI/helper and daemon work. The full sampler and cgroup-read duration distributions are separately labeled. No overhead subtraction or causal no-observer estimate is made. There is no Windows measurement here.

## Closure and retained capacity

The original driver reports an unexpected remaining full worker and performs exact failed-fixture cleanup after closing its controlled transports. Root's separate read-only closure observes empty Docker/container-volume inventory, 14 absent and two exited PID1-adopted browser zombies among 16 known identities, plus three unreadable other environments. It is not a whole-machine zero-process result.

The exact backend-settled lease `queue-1b52e8733b6479438946f479` for project `p-b6e34e18` / session `s-a692964a` still holds 1024 MiB and 1000 mCPU. This is not automatic product cleanup or capacity-release success. The failed fixture and its 375-file inventory remain unchanged; only a fresh supported recovery scenario may establish release. Cross-link LOCAL-016 rather than inventing a new product root cause. The original failed evidence must remain independent of any corrected helper or later run.
